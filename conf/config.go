package conf

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

type LogrusConfig struct {
	Level string `yaml:"level"`
}

type RsyncConfig struct {
	Host        string   `yaml:"host"`
	Port        int      `yaml:"port"`
	Username    string   `yaml:"username"`
	Password    string   `yaml:"password"`
	Timeout     int      `yaml:"timeout"`
	Space       string   `yaml:"space"`
	RootPath    string   `yaml:"root-path"`
	Compress    bool     `yaml:"compress"`
	AllowDelete bool     `yaml:"allow-delete"`
	FullSync    string   `yaml:"full-sync"`
	Excludes    []string `yaml:"excludes"`
}

type QueueConfig struct {
	RetryInterval int `yaml:"retry-interval"`
	QueueCapacity int `yaml:"queue-capacity"`
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
	if config.Rsync.Host == "" {
		return nil, fmt.Errorf("rsync.host is null")
	}
	if config.Rsync.Username == "" {
		return nil, fmt.Errorf("rsync.username is null")
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
	if config.Queue.RetryInterval == 0 {
		config.Queue.RetryInterval = 2000
	} else if config.Queue.RetryInterval < 0 {
		return nil, fmt.Errorf("rsync.retry-interval must be positive")
	}
	if config.Queue.QueueCapacity == 0 {
		config.Queue.QueueCapacity = 100
	} else if config.Queue.QueueCapacity < 0 {
		return nil, fmt.Errorf("rsync.queue-capacity must be positive")
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
