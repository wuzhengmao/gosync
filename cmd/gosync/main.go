package main

import (
	"flag"
	"fmt"
	"gosync/conf"
	"gosync/internal/rsync"
	"gosync/internal/watcher"
	"os"
	"runtime"
	"runtime/debug"
	"strings"

	"github.com/sirupsen/logrus"
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
	showVersion := flag.Bool("version", false, "show version information")
	flag.Parse()

	// 显示版本
	if *showVersion {
		fmt.Printf("Version: %s\nCommit: %s\nBuild Date: %s\n", conf.Version, commit, buildDate)
		os.Exit(0)
	}

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

	// 初始化RemoteSync
	rsync.Init(config.Rsync)

	// 初始化同步任务队列
	queue := &watcher.Queue{}
	queue.Init(config.Queue)

	// 初始化并启动监听
	err = watcher.Start(config.Rsync, queue)
	if err != nil {
		logrus.WithError(err).Fatalf("Start watcher error: %s", err.Error())
		os.Exit(2)
	}
}

type LogFormatter struct{}

func (f *LogFormatter) Format(entry *logrus.Entry) ([]byte, error) {
	funcName := "unknown"
	i := 1
	for ; ; i++ {
		pc, _, _, _ := runtime.Caller(i)
		funcName = runtime.FuncForPC(pc).Name()
		if !strings.HasPrefix(funcName, "github.com/sirupsen/logrus.") {
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
			// 跳过前8层堆栈信息
			stackTrace = strings.Join(strings.Split(stackTrace, "\n")[i*2:], "\n")
			logMessage += fmt.Sprintf("\nStack Trace: %s", stackTrace)
		}
	}
	return []byte(fmt.Sprintf("%s [%-5s] [%s] - %s\n", entry.Time.Format("2006-01-02 15:04:05"), strings.ToUpper(entry.Level.String()), funcName, logMessage)), nil
}
