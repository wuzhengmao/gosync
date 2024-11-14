package rsync

import (
	"fmt"
	"gosync/conf"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/sirupsen/logrus"
)

const excludesFile = "/tmp/rsync.excludes"
const secretFile = "/tmp/rsync.secret"

var config *conf.RsyncConfig

func Init(c *conf.RsyncConfig) {
	config = c
	if len(config.Excludes) > 0 {
		os.WriteFile(excludesFile, []byte(strings.Join(getExcludeArgs(), "\n")), 0600)
	}
	os.WriteFile(secretFile, []byte(config.Password), 0600)
}

func FullSync() bool {
	options := "-av"
	if config.Compress {
		options += "z"
	}
	options += "P"
	args := []string{options}
	if config.AllowDelete {
		args = append(args, "--delete", "--ignore-errors")
	}
	if len(config.Excludes) > 0 {
		args = append(args, fmt.Sprintf("--exclude-from=%s", excludesFile))
	}
	if config.Port > 0 && config.Port != 873 {
		args = append(args, fmt.Sprintf("--port=%d", config.Port))
	}
	args = append(args, fmt.Sprintf("--contimeout=%d", config.Timeout))
	args = append(args, config.RootPath, fmt.Sprintf("rsync://%s@%s/%s/", config.Username, config.Host, config.Space))
	logrus.Debugf("Execute: rsync %s", strings.Join(args, " "))
	args = append(args, fmt.Sprintf("--password-file=%s", secretFile))
	cmd := exec.Command("rsync", args...)
	if logrus.IsLevelEnabled(logrus.DebugLevel) {
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
	}
	err := cmd.Run()
	if err != nil {
		logrus.WithError(err).Errorf("Execute rsync failed.")
		return false
	} else {
		logrus.Info("Execute rsync successfully.")
		return true
	}
}

func Sync(path string) bool {
	options := "-av"
	if config.Compress {
		options += "z"
	}
	args := []string{options}
	if config.AllowDelete {
		args = append(args, "--delete", "--ignore-errors")
	}
	if len(config.Excludes) > 0 {
		args = append(args, fmt.Sprintf("--exclude-from=%s", excludesFile))
	}
	if config.Port > 0 && config.Port != 873 {
		args = append(args, fmt.Sprintf("--port=%d", config.Port))
	}
	args = append(args, fmt.Sprintf("--contimeout=%d", config.Timeout))
	args = append(args, config.RootPath+path, fmt.Sprintf("rsync://%s@%s/%s/%s", config.Username, config.Host, config.Space, path))
	logrus.Debugf("Execute: rsync %s", strings.Join(args, " "))
	args = append(args, fmt.Sprintf("--password-file=%s", secretFile))
	cmd := exec.Command("rsync", args...)
	if logrus.IsLevelEnabled(logrus.DebugLevel) {
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
	}
	err := cmd.Run()
	if err != nil {
		logrus.WithError(err).Errorf("Execute rsync failed.")
		return false
	} else {
		logrus.Info("Execute rsync successfully.")
		return true
	}
}

func Delete(path string) bool {
	if !config.AllowDelete {
		return true
	}
	parent := filepath.Dir(path) + "/"
	if strings.HasSuffix(path, "/") {
		parent = filepath.Dir(path[:len(path)-1]) + "/"
	}
	args := []string{"-av", "--delete", "--ignore-errors"}
	if len(config.Excludes) > 0 {
		args = append(args, fmt.Sprintf("--exclude-from=%s", excludesFile))
	}
	args = append(args, fmt.Sprintf("--include='%s'", filepath.Base(path)), "--exclude='*'")
	if config.Port > 0 && config.Port != 873 {
		args = append(args, fmt.Sprintf("--port=%d", config.Port))
	}
	args = append(args, fmt.Sprintf("--contimeout=%d", config.Timeout))
	args = append(args, config.RootPath+parent, fmt.Sprintf("rsync://%s@%s/%s/%s", config.Username, config.Host, config.Space, parent))
	logrus.Debugf("Execute: rsync %s", strings.Join(args, " "))
	args = append(args, fmt.Sprintf("--password-file=%s", secretFile))
	cmd := exec.Command("rsync", args...)
	if logrus.IsLevelEnabled(logrus.DebugLevel) {
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
	}
	err := cmd.Run()
	if err != nil {
		logrus.WithError(err).Errorf("Execute rsync failed.")
		return false
	} else {
		logrus.Info("Execute rsync successfully.")
		return true
	}
}

func getExcludeArgs() []string {
	args := []string{}
	for _, exclude := range config.Excludes {
		exclude = strings.TrimPrefix(exclude, "/")
		pattern := ""
		parts := strings.Split(exclude, "/")
		for i, part := range parts {
			if part == "**" {
				if i < len(parts)-1 {
					pattern += "**/" // **/a, a/**/b
				} else {
					if len(pattern) == 0 {
						pattern = "*" // ** -> *
					} else {
						// a/** -> a/
					}
				}
			} else if part == "*" {
				if i < len(parts)-1 {
					if i > 0 {
						pattern += "*/" // a/*/b
					} else {
						pattern += "/*/" // */a -> /*/a
					}
				} else {
					pattern += "*" // *, a/*
				}
			} else if part != "" {
				if i < len(parts)-1 {
					pattern += part + "/" // a/b
				} else {
					pattern += part // */a, **/a
				}
			} else {
				// a//b, a/b/
			}
		}
		args = append(args, "--exclude", pattern)
	}
	return args
}
