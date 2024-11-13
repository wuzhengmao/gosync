package watcher

import (
	"fmt"
	"gosync/conf"
	"gosync/internal/rsync"
	"strings"
	"time"

	"github.com/bmatcuk/doublestar/v4"
	"github.com/sirupsen/logrus"
)

const (
	CREATE = 1
	WRITE  = 2
	DELETE = 3
)

type Action struct {
	Method    int
	Path      string
	IsDir     bool
	Timestamp int64
}

func (action Action) String() string {
	str := ""
	switch action.Method {
	case CREATE:
		str += "CREATE "
	case WRITE:
		str += "WRITE "
	case DELETE:
		str += "DELETE "
	default:
		return "UNKNOWN"
	}
	str += action.Path
	if action.IsDir && !strings.HasSuffix(action.Path, "/") {
		str += "/"
	}
	return str
}

var config *conf.QueueConfig

type Queue []Action

func (queue *Queue) offer(method int, path string) {
	now := time.Now().UnixMilli()
	isDir := strings.HasSuffix(path, "/")
	ignore := false
	for _, action := range *queue {
		if method == CREATE {
			if action.Method == CREATE {
				if action.IsDir && isParent(action.Path, path) {
					ignore = true
					break
				}
			}
		} else if method == WRITE {
			if action.Method == CREATE {
				if action.IsDir && isParent(action.Path, path) {
					ignore = true
					break
				}
			} else if action.Method == WRITE {
				if action.Path == path {
					ignore = true
					break
				}
			}
		} else if method == DELETE {
			if action.Method == DELETE {
				if action.IsDir && isParent(action.Path, path) {
					ignore = true
					break
				}
			}
		}
	}
	if ignore {
		logrus.Debugf("%s (ignore)", logStr(method, path, isDir))
	} else {
		drops := ""
		for i := len(*queue) - 1; i >= 0; i-- {
			drop := false
			action := (*queue)[i]
			if method == CREATE {
				if isDir {
					if isParent(path, action.Path) {
						drop = true
					}
				} else {
					if !action.IsDir && path == action.Path {
						drop = true
					}
				}
			} else if method == WRITE {
				if !action.IsDir && path == action.Path {
					drop = true
				}
			} else if method == DELETE {
				if isDir {
					if isParent(path, action.Path) {
						drop = true
					}
				} else {
					if !action.IsDir && path == action.Path {
						drop = true
					}
				}
			}
			if drop {
				drops = fmt.Sprintf("\n    - %+v (drop)%s", action, drops)
				*queue = append((*queue)[:i], (*queue)[i+1:]...)
			}
		}
		logrus.Debugf("%s%s", logStr(method, path, isDir), drops)
		*queue = append(*queue, Action{Method: method, Path: path, IsDir: isDir, Timestamp: now})
	}
}

func isParent(folder string, path string) bool {
	ok, err := doublestar.Match(folder+"**", path)
	return ok && err == nil
}

func logStr(method int, path string, isDir bool) string {
	log := ""
	switch method {
	case CREATE:
		log = "Create "
	case WRITE:
		log = "Write "
	case DELETE:
		log = "Delete "
	}
	if isDir {
		log += "folder"
	} else {
		log += "file"
	}
	return log + ": " + path
}

func (queue *Queue) take() []Action {
	now := time.Now().UnixMilli()
	for i, action := range *queue {
		if now-action.Timestamp < 100 {
			actions := (*queue)[:i]
			*queue = (*queue)[i:]
			return actions
		}
	}
	actions := *queue
	*queue = []Action{}
	return actions
}

func (queue *Queue) Init(c conf.QueueConfig) {
	config = &c
}

func (queue *Queue) Start() {
	actions := []Action{}
	fullSync := true
	waitRetry := int64(0)
	for {
		actions = append(actions, queue.take()...)
		if fullSync {
			if len(actions) > 0 {
				log := ""
				for _, action := range actions {
					log += fmt.Sprintf("\n    - %+v (drop)", action)
				}
				logrus.Debugf("Ignore sync task, waiting for full sync execution.%s", log)
				actions = []Action{}
			}
		} else {
			if len(actions) > config.QueueCapacity {
				logrus.Warnf("The size of sync task queue exceeds %d, it will be converted to perform full sync.", config.QueueCapacity)
				fullSync = true
				actions = []Action{}
			}
		}
		if waitRetry == 0 || time.Now().UnixMilli() > waitRetry {
			waitRetry = 0
			if fullSync {
				if rsync.FullSync() {
					fullSync = false
				} else {
					waitRetry = time.Now().UnixMilli() + int64(config.RetryInterval)
					logrus.Infof("Waiting %d milliseconds to retry...", config.RetryInterval)
				}
			}
			if !fullSync && len(actions) > 0 {
				errorIndex := -1
				for i, action := range actions {
					log := "Starting "
					if action.Method != DELETE {
						log += "sync "
					} else {
						log += "delete "
					}
					if action.IsDir {
						log += "folder "
					} else {
						log += "file "
					}
					logrus.Infof("%s: %s ...", log, action.Path)
					ok := true
					if action.Method != DELETE {
						ok = rsync.Sync(action.Path)
					} else {
						ok = rsync.Delete(action.Path)
					}
					if !ok {
						errorIndex = i
						waitRetry = time.Now().UnixMilli() + int64(config.RetryInterval)
						break
					}
				}
				if errorIndex >= 0 {
					actions = actions[errorIndex:]
				} else {
					actions = []Action{}
				}
				if waitRetry > 0 {
					logrus.Infof("Waiting %d milliseconds to retry... (%d remaining tasks)", config.RetryInterval, len(actions))
				}
			}
		}
		time.Sleep(100 * time.Millisecond)
	}
}
