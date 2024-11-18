package watcher

import (
	"gosync/conf"
	"gosync/internal/rsync"
	"os"
	"path/filepath"
	"strings"
	"unsafe"

	"github.com/bmatcuk/doublestar/v4"
	"github.com/sirupsen/logrus"
	"golang.org/x/sys/unix"
)

func Start(config *conf.RsyncConfig, queue *Queue) error {
	watchDir := config.RootPath
	if !strings.HasSuffix(watchDir, "/") {
		watchDir += "/"
	}

	// 初始化 inotify
	fd, err := unix.InotifyInit()
	if err != nil {
		logrus.WithError(err).Error("Cannot initialize inotify.")
		return err
	}
	defer unix.Close(fd)

	// 创建一个映射表，将 watch descriptor (wd) 映射到目录路径
	wdToPath := make(map[int]string)

	// 添加根目录及其子目录到监听
	includes, err := rsync.GetWatchFolders()
	if err != nil {
		logrus.WithError(err).Error("Eval watch scope error")
		return err
	}
	err = addWatchRecursive(fd, watchDir, &includes, &config.Excludes, "", wdToPath)
	if err != nil {
		logrus.WithError(err).Errorf("Watch %s failed.", watchDir)
		return err
	}
	logrus.Infof("Watch %s started.", watchDir)

	// 开启同步任务
	go queue.Start()
	if config.FullSync == "startup" {
		queue.ScheduleFullSync()
	}

	// 创建用于接收事件的缓冲区
	buf := make([]byte, 4096)

	for {
		// 读取 inotify 事件
		n, err := unix.Read(fd, buf)
		if err != nil {
			logrus.WithError(err).Error("Read inotify event failed.")
		}

		// 解析事件
		var offset uint32
		for offset <= uint32(n-16) {
			raw := (*unix.InotifyEvent)(unsafe.Pointer(&buf[offset]))
			nameLen := uint32(raw.Len)
			name := ""
			if nameLen > 0 {
				name = strings.TrimRight(string(buf[offset+16:offset+16+nameLen-1]), "\x00")
			}

			// 从 wdToPath 映射表中获取事件目录的相对路径
			basePath, ok := wdToPath[int(raw.Wd)]
			if !ok {
				logrus.Warnf("Path not found for wd: %d", raw.Wd)
				continue
			}

			// 完整路径
			eventPath := filepath.Join(basePath, name)
			isDir := raw.Mask&unix.IN_ISDIR == unix.IN_ISDIR
			if isDir {
				eventPath += "/"
			}

			// 处理事件类型
			switch {
			case raw.Mask&unix.IN_CREATE == unix.IN_CREATE:
				// 如果创建的是目录，则递归监听该目录
				if isDir {
					if !isExclude(&config.Excludes, eventPath) {
						includes, err := rsync.GetWatchFolders()
						if err != nil {
							logrus.WithError(err).Error("Eval watch scope error")
						} else if shouldWatch(&includes, eventPath) {
							err = addWatchRecursive(fd, watchDir, &includes, &config.Excludes, eventPath, wdToPath)
							if err != nil {
								logrus.WithError(err).Errorf("Cannot watch folder: %s", eventPath)
							}
							queue.offer(CREATE, eventPath)
						}
					}
				}
			case raw.Mask&unix.IN_CLOSE_WRITE == unix.IN_CLOSE_WRITE:
				if !isExclude(&config.Excludes, eventPath) {
					queue.offer(WRITE, eventPath)
				}
			case raw.Mask&unix.IN_DELETE == unix.IN_DELETE:
				if config.AllowDelete && !isExclude(&config.Excludes, eventPath) {
					queue.offer(DELETE, eventPath)
				}
			case raw.Mask&unix.IN_MOVED_FROM == unix.IN_MOVED_FROM:
				if config.AllowDelete && !isExclude(&config.Excludes, eventPath) {
					queue.offer(DELETE, eventPath)
				}
			case raw.Mask&unix.IN_MOVED_TO == unix.IN_MOVED_TO:
				if !isExclude(&config.Excludes, eventPath) {
					queue.offer(CREATE, eventPath)
				}
			}

			offset += 16 + nameLen
		}
	}
}

// 递归添加目录及其子目录到 inotify 监听列表，并记录 wd 到路径的映射
func addWatchRecursive(fd int, watchDir string, includes *[]string, excludes *[]string, dir string, wdToPath map[int]string) error {
	return filepath.Walk(watchDir+dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		// 只对目录添加监听
		if info.IsDir() {
			relPath, _ := filepath.Rel(watchDir, path)
			relPath += "/"
			if shouldWatch(includes, relPath) && !isExclude(excludes, relPath) {
				wd, err := unix.InotifyAddWatch(fd, path, unix.IN_CREATE|unix.IN_MODIFY|unix.IN_CLOSE_WRITE|unix.IN_DELETE|unix.IN_MOVED_FROM|unix.IN_MOVED_TO)
				if err != nil {
					logrus.WithError(err).Errorf("Cannot watch folder: %s", relPath)
					return err
				}
				// 记录 wd 到目录路径的映射
				wdToPath[wd] = relPath
				logrus.Debugf("Watch folder: %s (wd: %d)", relPath, wd)
			}
		}
		return nil
	})
}

func isExclude(excludes *[]string, path string) bool {
	for _, exclude := range *excludes {
		if !strings.HasPrefix(exclude, "/") {
			exclude = "/" + exclude
		}
		if !strings.HasPrefix(path, "/") {
			path = "/" + path
		}
		if !strings.HasSuffix(exclude, "/") && strings.HasSuffix(path, "/") {
			path = path[:len(path)-1]
		}
		match, err := doublestar.Match(exclude, path)
		if match && err == nil {
			return true
		}
	}
	return false
}

func shouldWatch(includes *[]string, path string) bool {
	if includes == nil {
		return true
	}
	for _, include := range *includes {
		match, err := doublestar.Match(include+"**", path)
		if match && err == nil {
			return true
		}
		match, err = doublestar.Match(path+"**", include)
		if match && err == nil {
			return true
		}
	}
	return false
}
