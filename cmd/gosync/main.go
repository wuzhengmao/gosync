package main

import (
	"flag"
	"fmt"
	"gosync/conf"
	"gosync/internal/job"
	"gosync/internal/rsync"
	"gosync/internal/watcher"
	"log/syslog"
	"os"
	"path/filepath"
	"runtime"
	"runtime/debug"
	"strings"

	"github.com/sevlyar/go-daemon"
	"github.com/sirupsen/logrus"
	syslogHook "github.com/sirupsen/logrus/hooks/syslog"
	"gopkg.in/natefinch/lumberjack.v2"
)

// 定义编译时变量
var (
	commit    string
	buildDate string
)

func main() {
	logrus.SetFormatter(&LogFormatter{})
	logrus.SetLevel(logrus.InfoLevel)

	// 处理命令行参数
	configFile := flag.String("config", "", "configuration file")
	isDaemon := flag.Bool("daemon", false, "run as a service")
	showVersion := flag.Bool("version", false, "show version information")
	flag.Parse()

	// 显示banner
	fmt.Print(`     ___    ___                           
    / _ \  /___\  ___  _   _  _ __    ___ 
   / /_\/ //  // / __|| | | || '_ \  / __|
  / /_\\ / \_//  \__ \| |_| || | | || (__ 
  \____/ \___/   |___/ \__, ||_| |_| \___|
                       |___/              

`)

	// 显示版本
	version := conf.Version
	if commit != "" {
		version += "-" + commit
	}
	fmt.Printf("gosync %s (%s %s/%s)\n", version, runtime.Version(), runtime.GOOS, runtime.GOARCH)
	if *showVersion {
		if buildDate != "" {
			fmt.Printf("built on %s\n", buildDate)
		}
		fmt.Printf("Copyright (c) 2024 Mingy, MTI license\n")
		fmt.Println()
		os.Exit(0)
	}
	fmt.Println()

	// 加载配置
	config, err := conf.Load(*configFile)
	if err != nil {
		logrus.WithError(err).Fatalf("Load config error: %s", err.Error())
		os.Exit(1)
	}

	// 初始化日志
	switch config.Logrus.Level {
	case "VERBOSE", "TRACE":
		logrus.SetLevel(logrus.TraceLevel)
	case "DEBUG":
		logrus.SetLevel(logrus.DebugLevel)
	case "INFO":
		logrus.SetLevel(logrus.InfoLevel)
	case "WARN":
		logrus.SetLevel(logrus.WarnLevel)
	case "ERROR":
		logrus.SetLevel(logrus.ErrorLevel)
	case "FATAL":
		logrus.SetLevel(logrus.FatalLevel)
	}
	if config.Logrus.Output == "file" {
		logFile := config.Logrus.File.Path
		if !filepath.IsAbs(logFile) {
			workDir, _ := os.Getwd()
			logFile = workDir + "/" + logFile
		}
		logger := &lumberjack.Logger{
			Filename:   logFile,
			MaxSize:    config.Logrus.File.MaxSize,
			MaxBackups: config.Logrus.File.MaxBackups,
			MaxAge:     config.Logrus.File.MaxAge,
			Compress:   config.Logrus.File.Compress,
		}
		logrus.SetOutput(logger)
	} else if config.Logrus.Output == "syslog" {
		hook, err := syslogHook.NewSyslogHook("", "", syslog.LOG_INFO|syslog.LOG_LOCAL0, "gosync")
		if err != nil {
			logrus.Fatalf("Failed to connect to syslog: %v", err)
		}
		logrus.AddHook(hook)
		blackhole, _ := os.OpenFile("/dev/null", os.O_WRONLY, 0644)
		logrus.SetOutput(blackhole)
	}

	// 后台进程
	if *isDaemon {
		// 创建一个新的守护进程
		ctx := &daemon.Context{
			PidFileName: "/run/gosync.pid", // PID文件保存位置
			PidFilePerm: 0644,
		}
		// 在后台启动进程
		child, err := ctx.Reborn()
		if err != nil {
			logrus.WithError(err).Fatalf("Start service error: %s", err.Error())
			os.Exit(2)
		}
		if child != nil {
			// 如果我们是子进程（即守护进程本身）
			logrus.Info("GO sync run as a service.")
			return
		}
		defer ctx.Release() // 当程序结束时释放守护进程资源
	}

	// 初始化RemoteSync
	err = rsync.Init(config)
	if err != nil {
		logrus.WithError(err).Fatalf("Initialize rsync error: %s", err.Error())
		os.Exit(3)
	}

	// 初始化同步任务队列
	queue := watcher.CreateQueue(&config.Queue)

	// 启动定时任务
	err = job.Start(config, &queue)
	if err != nil {
		logrus.WithError(err).Fatalf("Start scheduled job error: %s", err.Error())
		os.Exit(4)
	}
	defer job.Stop()

	// 初始化并启动监听
	err = watcher.Start(&config.Rsync, &queue)
	if err != nil {
		logrus.WithError(err).Fatalf("Start watcher error: %s", err.Error())
		os.Exit(5)
	}
}

type LogFormatter struct{}

func (f *LogFormatter) Format(entry *logrus.Entry) ([]byte, error) {
	funcName := "unknown"
	i := 1
	for ; ; i++ {
		pc, _, _, _ := runtime.Caller(i)
		funcName = runtime.FuncForPC(pc).Name()
		if !strings.HasPrefix(funcName, "github.com/sirupsen/logrus") && !strings.HasPrefix(funcName, "gosync/internal/job.CronLogrus.") {
			break
		}
	}
	// 如果有错误，输出错误信息和堆栈信息
	logMessage := entry.Message
	if err, ok := entry.Data["error"]; ok {
		if err != nil {
			logMessage += fmt.Sprintf("\nError: %s", err)
			// 获取堆栈信息
			stackTrace := string(debug.Stack())
			// 跳过调用日志库的堆栈信息
			stackTrace = strings.Join(strings.Split(stackTrace, "\n")[(i+1)*2+1:], "\n")
			logMessage += fmt.Sprintf("\nStack Trace: \n%s", stackTrace)
		}
	}
	return []byte(fmt.Sprintf("%s [%-5s] [%s] - %s\n", entry.Time.Format("2006-01-02 15:04:05"), strings.ToUpper(entry.Level.String()), funcName, logMessage)), nil
}
