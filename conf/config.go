package conf

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

type LogrusConfig struct {
	Level  string        `yaml:"level"`
	Output string        `yaml:"output"`
	File   LogFileConfig `yaml:"file"`
}

type LogFileConfig struct {
	Path       string `yaml:"path"`
	MaxSize    int    `yaml:"max-size"`
	MaxBackups int    `yaml:"max-backups"`
	MaxAge     int    `yaml:"max-age"`
	Compress   bool   `yaml:"compress"`
}

type RsyncConfig struct {
	Host           string   `yaml:"host"`
	Port           int      `yaml:"port"`
	Username       string   `yaml:"username"`
	Password       string   `yaml:"password"`
	Timeout        string   `yaml:"timeout"`
	IOTimeout      string   `yaml:"io-timeout"`
	Space          string   `yaml:"space"`
	RootPath       string   `yaml:"root-path"`
	WatchScopeEval string   `yaml:"watch-scope-eval"`
	Compress       bool     `yaml:"compress"`
	AllowDelete    bool     `yaml:"allow-delete"`
	FullSync       string   `yaml:"full-sync"`
	Excludes       []string `yaml:"excludes"`
}

type QueueConfig struct {
	RetryInterval string `yaml:"retry-interval"`
	Capacity      int    `yaml:"capacity"`
}

type JobConfig struct {
	Cron    string `yaml:"cron"`
	Command string `yarm:"command"`
}

type Config struct {
	Dir    string
	Logrus LogrusConfig `yaml:"log"`
	Rsync  RsyncConfig  `yaml:"rsync"`
	Queue  QueueConfig  `yaml:"queue"`
	Jobs   []JobConfig  `yaml:"jobs"`
}

func Load(filename string) (*Config, error) {
	configFile := ""
	if filepath.IsAbs(filename) {
		configFile = filename
	} else {
		path, err := os.Getwd()
		if err == nil {
			configFile = find(path, filename)
		}
		if configFile == "" {
			path, err := os.Executable()
			if err == nil {
				configFile = find(path, filename)
			}
		}
		if configFile == "" {
			configFile = find("/etc", filename)
		}
		if configFile == "" {
			configFile = find("/etc/gosync", filename)
		}
		if configFile == "" {
			if filename == "" {
				return nil, fmt.Errorf("gosync.yml not found")
			} else {
				return nil, fmt.Errorf("%s not found", filename)
			}
		}
	}

	data, err := os.ReadFile(configFile)
	if err != nil {
		return nil, err
	}

	var config Config
	err = yaml.Unmarshal(data, &config)
	if err != nil {
		return nil, err
	}

	config.Dir = filepath.Dir(configFile)
	if config.Logrus.Level == "" {
		config.Logrus.Level = "INFO"
	} else {
		config.Logrus.Level = strings.ToUpper(config.Logrus.Level)
	}
	if config.Logrus.Output == "" {
		config.Logrus.Output = "stdout"
	} else {
		config.Logrus.Output = strings.ToLower(config.Logrus.Output)
		if config.Logrus.Output != "stdout" && config.Logrus.Output != "syslog" && config.Logrus.Output != "file" {
			return nil, fmt.Errorf("log.output must be stdout syslog or file")
		}
		if config.Logrus.Output == "file" {
			if config.Logrus.File.Path == "" {
				return nil, fmt.Errorf("log.file.path is null")
			}
		}
	}
	if config.Rsync.Host == "" {
		return nil, fmt.Errorf("rsync.host is null")
	}
	if config.Rsync.Username == "" {
		return nil, fmt.Errorf("rsync.username is null")
	}
	if config.Rsync.Timeout != "" {
		_, err := time.ParseDuration(config.Rsync.Timeout)
		if err != nil {
			return nil, fmt.Errorf("rsync.timeout format is invalid")
		}
	}
	if config.Rsync.Space == "" {
		return nil, fmt.Errorf("rsync.space is null")
	}
	if config.Rsync.RootPath == "" {
		return nil, fmt.Errorf("rsync.root-path is null")
	} else if !strings.HasPrefix(config.Rsync.RootPath, "/") {
		return nil, fmt.Errorf("rsync.root-path must be a absolute path")
	} else if !strings.HasSuffix(config.Rsync.RootPath, "/") {
		config.Rsync.RootPath += "/"
	}
	if config.Rsync.FullSync == "" {
		config.Rsync.FullSync = "startup"
	} else {
		config.Rsync.FullSync = strings.ToLower(config.Rsync.FullSync)
		if config.Rsync.FullSync == "false" {
			config.Rsync.FullSync = "none"
		}
	}
	if config.Queue.RetryInterval == "" {
		config.Queue.RetryInterval = "2s"
	} else {
		_, err := time.ParseDuration(config.Queue.RetryInterval)
		if err != nil {
			return nil, fmt.Errorf("queue.retry-interval format is invalid")
		}
	}
	if config.Queue.Capacity == 0 {
		config.Queue.Capacity = 100
	} else if config.Queue.Capacity < 0 {
		return nil, fmt.Errorf("queue.capacity must be positive")
	}
	for _, job := range config.Jobs {
		if job.Cron == "" {
			return nil, fmt.Errorf("job.cron is null")
		}
		if job.Command == "" {
			return nil, fmt.Errorf("job.command is null")
		}
	}

	return &config, nil
}

func find(path string, name string) string {
	if !strings.HasSuffix(path, "/") {
		path += "/"
	}
	if name == "" {
		file := path + "gosync.yaml"
		info, err := os.Stat(file)
		if err == nil && !info.IsDir() {
			return file
		}
		file = path + "gosync.yml"
		info, err = os.Stat(file)
		if err == nil && !info.IsDir() {
			return file
		}
	} else {
		file := path + name
		info, err := os.Stat(file)
		if err == nil && !info.IsDir() {
			return file
		}
	}
	return ""
}
