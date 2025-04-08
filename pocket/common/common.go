package common

import (
	"runtime"
	"os"
	"fmt"
	"io"
	"strings"

	"github.com/sirupsen/logrus"
	"gopkg.in/natefinch/lumberjack.v2"
)

type LogFormatter struct {}
func (f *LogFormatter) Format(entry *logrus.Entry) ([]byte, error) {
	timestamp := entry.Time.Format("2006-01-02 15:04:05")
	level := strings.ToUpper(entry.Level.String())
	msg := entry.Message
	
	// 移除消息末尾的换行符(如果存在)
	msg = strings.TrimSuffix(msg, "\n")
	
	// 构建自定义格式
	logLine := fmt.Sprintf("%s [%s]\t%s\n", timestamp, level, msg)
	return []byte(logLine), nil
}

func InitLog(path string) (*logrus.Logger, error) {
	if len(path) == 0 {
		path = "marude"
	}

	var log_path string

	switch(runtime.GOOS) {
	case "windows":
		sysdata_path := os.Getenv("ProgramData")
		log_path = fmt.Sprintf("%s/%s", sysdata_path, path)
	case "linux":
		log_path = fmt.Sprintf("/var/log/%s", path)
	case "darwin":
		log_path = fmt.Sprintf("/Library/Application Support/%s", path)
	}

	err := os.MkdirAll(log_path, os.ModeDir)

	if err != nil && !os.IsExist(err) {
		return nil, err
	}

	logger := logrus.New()

	logger.SetFormatter(&LogFormatter{})

	logger.SetOutput(io.MultiWriter(os.Stdout, &lumberjack.Logger{
		Filename:		fmt.Sprintf("%s/marude.log", log_path),
		MaxSize:		100,
		MaxBackups:		5,
		MaxAge:			30,
		Compress:		true,
	}))


	return logger, nil
}