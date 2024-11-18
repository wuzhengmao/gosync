package watcher

import (
	"fmt"
	"gosync/conf"
	"gosync/internal/rsync"
	"math"
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

type Queue struct {
	config   *conf.QueueConfig
	actions  *[]Action
	fullSync bool
}

func CreateQueue(c *conf.QueueConfig) Queue {
	return Queue{
		config:   c,
		actions:  &[]Action{},
		fullSync: false,
	}
}

func (queue *Queue) offer(method int, path string) {
	now := time.Now().UnixMilli()
	isDir := strings.HasSuffix(path, "/")
	ignore := false
	for _, action := range *queue.actions {
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
		for i := len(*queue.actions) - 1; i >= 0; i-- {
			drop := false
			action := (*queue.actions)[i]
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
				*queue.actions = append((*queue.actions)[:i], (*queue.actions)[i+1:]...)
			}
		}
		logrus.Debugf("%s%s", logStr(method, path, isDir), drops)
		*queue.actions = append((*queue.actions), Action{Method: method, Path: path, IsDir: isDir, Timestamp: now})
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
	for i, action := range *queue.actions {
		if now-action.Timestamp < 100 {
			actions := (*queue.actions)[:i]
			*queue.actions = (*queue.actions)[i:]
			return actions
		}
	}
	actions := *queue.actions
	*queue.actions = []Action{}
	return actions
}

func (queue *Queue) Start() {
	actions := []Action{}
	retryInterval, _ := time.ParseDuration(queue.config.RetryInterval)
	waitRetry := int64(0)
	for {
		actions = append(actions, queue.take()...)
		if queue.fullSync {
			if len(actions) > 0 {
				log := ""
				for _, action := range actions {
					log += fmt.Sprintf("\n    - %+v (drop)", action)
				}
				logrus.Debugf("Ignore sync task, waiting for full sync execution.%s", log)
				actions = []Action{}
			}
		} else {
			if len(actions) > queue.config.Capacity {
				logrus.Warnf("The size of sync task queue exceeds %d, it will be converted to perform full sync.", queue.config.Capacity)
				queue.fullSync = true
				actions = []Action{}
			}
		}
		if waitRetry == 0 || time.Now().UnixMilli() > waitRetry {
			waitRetry = 0
			if queue.fullSync {
				if rsync.FullSync() {
					queue.fullSync = false
				} else {
					waitRetry = time.Now().Add(retryInterval).UnixMilli()
					logrus.Infof("Waiting %d seconds to retry...", int(math.Ceil(retryInterval.Seconds())))
				}
			}
			if !queue.fullSync && len(actions) > 0 {
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
						waitRetry = time.Now().Add(retryInterval).UnixMilli()
						break
					}
				}
				if errorIndex >= 0 {
					actions = actions[errorIndex:]
				} else {
					actions = []Action{}
				}
				if waitRetry > 0 {
					logrus.Infof("Waiting %d seconds to retry... (%d remaining tasks)", int(math.Ceil(retryInterval.Seconds())), len(actions))
				}
			}
		}
		time.Sleep(100 * time.Millisecond)
	}
}

func (queue *Queue) ScheduleFullSync() {
	logrus.Info("Scheduling to perform full sync...")
	queue.fullSync = true
}
