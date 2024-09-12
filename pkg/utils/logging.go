package utils

import (
	"github.com/sirupsen/logrus"
)

func InitLogging(appName string, logLevel string) (*logrus.Entry, error) {
	log := logrus.New()
	log.SetFormatter(&logrus.JSONFormatter{
		FieldMap: logrus.FieldMap{
			logrus.FieldKeyTime:  "@timestamp",
			logrus.FieldKeyLevel: "level",
			logrus.FieldKeyMsg:   "message",
		},
	})

	level, err := logrus.ParseLevel(logLevel)
	if err != nil {
		log.WithError(err).Error("Error parsing log level")
		return nil, err
	}

	log.SetLevel(level)

	return log.WithFields(logrus.Fields{
		"app": appName,
	}), nil
}
