package job

import (
	"fmt"
	"gosync/conf"
	"gosync/internal/watcher"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/robfig/cron/v3"
	"github.com/sirupsen/logrus"
)

type CronLogrus struct{}

func (logger CronLogrus) Info(msg string, keysAndValues ...interface{}) {
	logrus.Debugf(msg, keysAndValues...)
}

func (logger CronLogrus) Error(err error, msg string, keysAndValues ...interface{}) {
	logrus.WithError(err).Errorf(msg, keysAndValues...)
}

var c *cron.Cron
var config *conf.Config
var queue *watcher.Queue

func Start(cf *conf.Config, q *watcher.Queue) error {
	config = cf
	c = cron.New(cron.WithLogger(CronLogrus{}))
	queue = q
	if cf.Rsync.FullSync != "startup" && cf.Rsync.FullSync != "none" {
		cf.Jobs = append(cf.Jobs, conf.JobConfig{
			Cron:    cf.Rsync.FullSync,
			Command: "full-sync",
		})
	}
	for _, job := range cf.Jobs {
		err := Add(job.Cron, job.Command)
		if err != nil {
			return err
		}
	}
	c.Start()
	logrus.Infof("Total of %d scheduled jobs started.", len(cf.Jobs))
	return nil
}

func Stop() {
	c.Stop()
	logrus.Info("Scheduled jobs stopped.")
}

func Add(cron string, command string) error {
	if strings.HasPrefix(strings.ToLower(cron), "@after ") {
		after, err := time.ParseDuration(cron[7:])
		if err != nil {
			return fmt.Errorf("failed to parse after %s: %s", cron, err)
		}
		time.AfterFunc(after, func() {
			run(command)
		})
	} else {
		_, err := c.AddFunc(cron, func() {
			run(command)
		})
		if err != nil {
			return err
		}
	}
	return nil
}

func run(command string) bool {
	if strings.ToLower(command) == "full-sync" {
		queue.ScheduleFullSync()
		return true
	} else {
		logrus.Infof("Run job: %s", command)
		args := strings.Split(command, " ")
		cmd := exec.Command(args[0], args[1:]...)
		cmd.Dir = config.Dir
		cmd.Env = append(os.Environ(),
			"RSYNC_HOST="+config.Rsync.Host,
			"RSYNC_PORT="+string(config.Rsync.Port),
			"RSYNC_USERNAME="+config.Rsync.Username,
			"RSYNC_PASSWORD="+config.Rsync.Password,
			"RSYNC_SPACE="+config.Rsync.Space,
			"RSYNC_ROOT_PATH="+config.Rsync.RootPath,
		)
		if logrus.IsLevelEnabled(logrus.DebugLevel) {
			cmd.Stdout = os.Stdout
			cmd.Stderr = os.Stderr
		}
		err := cmd.Run()
		if err != nil {
			logrus.WithError(err).Error("Run job failed.")
			return false
		} else {
			logrus.Info("Run job successfully.")
			return true
		}
	}
}
