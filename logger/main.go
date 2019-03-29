package logger

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/sirupsen/logrus"
)

func logrusFields(fields customLogFields) *logrus.Fields {
	if hostName == "" {
		hostName, _ = os.Hostname()
	}
	logrusFields := logrus.Fields{
		"@hostname":       hostName,
		"@vertical":       fields.Vertical,
		"service-name":    "chaoskube",
		"killed-pod-name": fields.KilledPodName,
		"killed-service":  fields.KilledService,
		"chaos-action":    fields.ChaosAction,
		"service-version": "1.0",
		"type":            "service",
		"retention":       "technical",
	}
	return &logrusFields
}

type customLogFields struct {
	Vertical      string
	KilledPodName string
	KilledService string
	ChaosAction   string
}

var hostName = ""

// WithCustomFields adds more details to log lines
func WithCustomFields(victim string) *logrus.Entry {
	custom := customLogFields{}
	splitted := strings.Split(victim, "-")
	if len(splitted) != 4 {
		custom.KilledPodName = victim
	} else {
		custom.Vertical = splitted[0]
		custom.KilledService = fmt.Sprintf("%s-%s", splitted[0], splitted[1])
		custom.KilledPodName = victim
		custom.ChaosAction = "KILL"
	}

	return logrus.WithFields(*logrusFields(custom))
}

func init() {
	logrus.SetFormatter(&logrus.JSONFormatter{
		FieldMap:        logrus.FieldMap{logrus.FieldKeyTime: "@timestamp"},
		TimestampFormat: time.RFC3339Nano})
	logrus.SetOutput(os.Stdout)
	logrus.SetLevel(logrus.InfoLevel)
}
