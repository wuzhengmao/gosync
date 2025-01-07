package rsync

import (
	"fmt"
	"gosync/conf"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/sirupsen/logrus"
)

var config *conf.RsyncConfig
var workdir = ""
var excludesFile = ""
var secretFile = ""

func Init(c *conf.Config) error {
	config = &c.Rsync
	workdir = c.Dir
	if len(config.Excludes) > 0 {
		excludesFile = "/tmp/rsync.excludes"
		err := os.WriteFile(excludesFile, []byte(strings.Join(getExcludes(), "\n")), 0600)
		if err != nil {
			return err
		}
	}
	if config.Password != "" {
		secretFile = "/tmp/rsync.secret"
		err := os.WriteFile(secretFile, []byte(config.Password), 0600)
		if err != nil {
			return err
		}
	}
	return nil
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
	if excludesFile != "" {
		args = append(args, fmt.Sprintf("--exclude-from=%s", excludesFile))
	}
	includeFiles, err := getIncludes()
	if err != nil {
		logrus.WithError(err).Error("Execute rsync failed.")
		return false
	} else if includeFiles != "" {
		args = append(args, fmt.Sprintf("--include-from=%s", includeFiles), "--exclude='*'")
	}
	if config.Port > 0 && config.Port != 873 {
		args = append(args, fmt.Sprintf("--port=%d", config.Port))
	}
	if config.Timeout != "" {
		timeout, _ := time.ParseDuration(config.Timeout)
		args = append(args, fmt.Sprintf("--contimeout=%d", int(math.Ceil(timeout.Seconds()))))
	}
	args = append(args, config.RootPath, fmt.Sprintf("rsync://%s@%s/%s/", config.Username, config.Host, config.Space))
	logrus.Debugf("Execute: rsync %s", strings.Join(args, " "))
	if secretFile != "" {
		args = append(args, fmt.Sprintf("--password-file=%s", secretFile))
	}
	cmd := exec.Command("rsync", args...)
	if logrus.IsLevelEnabled(logrus.DebugLevel) {
		cmd.Stdout = logrus.StandardLogger().Out
		cmd.Stderr = logrus.StandardLogger().Out
	}
	err = cmd.Run()
	if err != nil {
		logrus.WithError(err).Error("Execute rsync failed.")
		return false
	} else {
		logrus.Info("Execute rsync successfully.")
		return true
	}
}

func Sync(path string) bool {
	_, err := os.Stat(config.RootPath + path)
	if err != nil {
		logrus.Warn("Ignore rsync because path is not exists.")
		return true
	}
	options := "-av"
	if config.Compress {
		options += "z"
	}
	args := []string{options}
	if config.AllowDelete {
		args = append(args, "--delete", "--ignore-errors")
	}
	if excludesFile != "" {
		args = append(args, fmt.Sprintf("--exclude-from=%s", excludesFile))
	}
	if config.Port > 0 && config.Port != 873 {
		args = append(args, fmt.Sprintf("--port=%d", config.Port))
	}
	if config.Timeout != "" {
		timeout, _ := time.ParseDuration(config.Timeout)
		args = append(args, fmt.Sprintf("--contimeout=%d", int(math.Ceil(timeout.Seconds()))))
	}
	args = append(args, config.RootPath+path, fmt.Sprintf("rsync://%s@%s/%s/%s", config.Username, config.Host, config.Space, path))
	logrus.Debugf("Execute: rsync %s", strings.Join(args, " "))
	if secretFile != "" {
		args = append(args, fmt.Sprintf("--password-file=%s", secretFile))
	}
	cmd := exec.Command("rsync", args...)
	if logrus.IsLevelEnabled(logrus.DebugLevel) {
		cmd.Stdout = logrus.StandardLogger().Out
		cmd.Stderr = logrus.StandardLogger().Out
	}
	err = cmd.Run()
	if err != nil {
		logrus.WithError(err).Error("Execute rsync failed.")
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
	if excludesFile != "" {
		args = append(args, fmt.Sprintf("--exclude-from=%s", excludesFile))
	}
	args = append(args, fmt.Sprintf("--include='%s'", filepath.Base(path)), "--exclude='*'")
	if config.Port > 0 && config.Port != 873 {
		args = append(args, fmt.Sprintf("--port=%d", config.Port))
	}
	if config.Timeout != "" {
		timeout, _ := time.ParseDuration(config.Timeout)
		args = append(args, fmt.Sprintf("--contimeout=%d", int(math.Ceil(timeout.Seconds()))))
	}
	args = append(args, config.RootPath+parent, fmt.Sprintf("rsync://%s@%s/%s/%s", config.Username, config.Host, config.Space, parent))
	logrus.Debugf("Execute: rsync %s", strings.Join(args, " "))
	if secretFile != "" {
		args = append(args, fmt.Sprintf("--password-file=%s", secretFile))
	}
	cmd := exec.Command("rsync", args...)
	if logrus.IsLevelEnabled(logrus.DebugLevel) {
		cmd.Stdout = logrus.StandardLogger().Out
		cmd.Stderr = logrus.StandardLogger().Out
	}
	err := cmd.Run()
	if err != nil {
		logrus.WithError(err).Error("Execute rsync failed.")
		return false
	} else {
		logrus.Info("Execute rsync successfully.")
		return true
	}
}

func GetWatchFolders() ([]string, error) {
	if config.WatchScopeEval == "" {
		return nil, nil
	}
	v := strings.Split(config.WatchScopeEval, " ")
	cmd := exec.Command(v[0], v[1:]...)
	cmd.Dir = workdir
	cmd.Env = append(os.Environ(),
		"RSYNC_ROOT_PATH="+config.RootPath,
	)
	stdout, err := cmd.CombinedOutput()
	str := string(stdout)
	if err != nil {
		logrus.WithError(err).Errorf("Get watch folders error: %s", str)
		return nil, err
	} else {
		logrus.Debugf("Get watch folders returns: \n%s", str)
		folders := strings.Split(str, "\n")
		out := []string{}
		for _, folder := range folders {
			folder = strings.TrimSpace(folder)
			if folder != "" {
				if folder == "/" {
					return nil, nil
				} else {
					folder = strings.TrimPrefix(folder, "/")
					if !strings.HasSuffix(folder, "/") {
						folder += "/"
					}
				}
				out = append(out, folder)
			}
		}
		return out, nil
	}
}

func getIncludes() (string, error) {
	folders, err := GetWatchFolders()
	if err != nil {
		return "", err
	} else if folders == nil {
		return "", nil
	} else {
		includesFile := "/tmp/rsync.includes"
		err := os.WriteFile(includesFile, []byte(strings.Join(folders, "\n")), 0600)
		return includesFile, err
	}
}

func getExcludes() []string {
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
