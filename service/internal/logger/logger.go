package logger

import (
	"os"
	"time"

	"github.com/sirupsen/logrus"
)

type Logger struct{ *logrus.Logger }
type Fields map[string]interface{}

func New() *Logger {
	log := logrus.New()

	env := os.Getenv("ENV")
	if env == "production" {
		log.SetFormatter(&logrus.JSONFormatter{
			TimestampFormat: time.RFC3339Nano,
			FieldMap: logrus.FieldMap{
				logrus.FieldKeyTime:  "@timestamp",
				logrus.FieldKeyLevel: "severity",
				logrus.FieldKeyMsg:   "message",
				logrus.FieldKeyFunc:  "function",
			},
		})
	} else {
		log.SetFormatter(&logrus.TextFormatter{
			TimestampFormat: time.RFC3339,
			FullTimestamp:   true,
		})
	}

	level := os.Getenv("LOG_LEVEL")
	if level == "" {
		level = "info"
	}
	if lvl, err := logrus.ParseLevel(level); err == nil {
		log.SetLevel(lvl)
	}

	log.SetOutput(os.Stdout)
	return &Logger{Logger: log}
}
