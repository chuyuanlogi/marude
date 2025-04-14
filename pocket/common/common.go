package common

import (
	"fmt"
	"io"
	"os"
	"runtime"
	"strings"

	"github.com/sirupsen/logrus"
	"gopkg.in/natefinch/lumberjack.v2"
)

type LogFormatter struct{}

func (f *LogFormatter) Format(entry *logrus.Entry) ([]byte, error) {
	timestamp := entry.Time.Format("2006-01-02 15:04:05")
	level := strings.ToUpper(entry.Level.String())
	msg := entry.Message
	msg = strings.TrimSuffix(msg, "\n")
	logLine := fmt.Sprintf("%s [%s]\t%s\n", timestamp, level, msg)
	return []byte(logLine), nil
}

func InitLog(path string) (*logrus.Logger, error) {
	if len(path) == 0 {
		path = "marude"
	}

	var log_path string

	switch runtime.GOOS {
	case "windows":
		sysdata_path := os.Getenv("ProgramData")
		log_path = fmt.Sprintf("%s/%s", sysdata_path, path)
	case "linux":
		log_path = fmt.Sprintf("/var/log/%s", path)
	case "darwin":
		log_path = fmt.Sprintf("/Library/Application Support/%s", path)
	}

	ljack := &lumberjack.Logger{
		Filename:   fmt.Sprintf("%s/marude.log", log_path),
		MaxSize:    15,
		MaxBackups: 5,
		MaxAge:     30,
		Compress:   true,
	}
	var writer io.Writer

	switch runtime.GOOS {
	case "windows":
		writer = io.MultiWriter(ljack)
	case "linux", "darwin":
		writer = io.MultiWriter(os.Stdout, ljack)
	}

	err := os.MkdirAll(log_path, os.ModeDir)

	if err != nil && !os.IsExist(err) {
		return nil, err
	}

	logger := logrus.New()

	logger.SetFormatter(&LogFormatter{})

	logger.SetOutput(writer)

	return logger, nil
}
