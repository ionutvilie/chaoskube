package internal

import (
	"os"
	"time"

	log "github.com/sirupsen/logrus"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"

	"github.com/ionutvilie/chaoskube/chaoskube"
	"github.com/ionutvilie/chaoskube/util"
)

type ChaoskubeConfig struct {
	Labels             string
	Annotations        string
	Namespaces         string
	ExcludedWeekdays   string
	ExcludedTimesOfDay string
	ExcludedDaysOfYear string
	Timezone           string
	Master             string
	Kubeconfig         string
	DryRun             bool
	HTTPServer         bool
	Debug              bool
	Interval           time.Duration
}

func (ckFC *ChaoskubeConfig) NewMonkey() *chaoskube.Chaoskube {

	client, err := ckFC.newK8sClient()
	if err != nil {
		log.WithField("err", err).Fatal("failed to connect to cluster")
	}

	var (
		labelSelector = parseSelector(ckFC.Labels)
		annotations   = parseSelector(ckFC.Annotations)
		namespaces    = parseSelector(ckFC.Namespaces)
	)

	log.WithFields(log.Fields{
		"labels":      labelSelector,
		"annotations": annotations,
		"namespaces":  namespaces,
	}).Info("setting pod filter")

	parsedWeekdays := util.ParseWeekdays(ckFC.ExcludedWeekdays)
	parsedTimesOfDay, err := util.ParseTimePeriods(ckFC.ExcludedTimesOfDay)
	if err != nil {
		log.WithFields(log.Fields{
			"timesOfDay": ckFC.ExcludedTimesOfDay,
			"err":        err,
		}).Fatal("failed to parse times of day")
	}
	parsedDaysOfYear, err := util.ParseDays(ckFC.ExcludedDaysOfYear)
	if err != nil {
		log.WithFields(log.Fields{
			"daysOfYear": ckFC.ExcludedDaysOfYear,
			"err":        err,
		}).Fatal("failed to parse days of year")
	}

	log.WithFields(log.Fields{
		"weekdays":   parsedWeekdays,
		"timesOfDay": parsedTimesOfDay,
		"daysOfYear": formatDays(parsedDaysOfYear),
	}).Info("setting quiet times")

	parsedTimezone, err := time.LoadLocation(ckFC.Timezone)
	if err != nil {
		log.WithFields(log.Fields{
			"timeZone": ckFC.Timezone,
			"err":      err,
		}).Fatal("failed to detect time zone")
	}
	timezoneName, offset := time.Now().In(parsedTimezone).Zone()

	log.WithFields(log.Fields{
		"name":     timezoneName,
		"location": parsedTimezone,
		"offset":   offset / int(time.Hour/time.Second),
	}).Info("setting timezone")

	ck := chaoskube.New(
		client,
		labelSelector,
		annotations,
		namespaces,
		parsedWeekdays,
		parsedTimesOfDay,
		parsedDaysOfYear,
		parsedTimezone,
		log.StandardLogger(),
		ckFC.DryRun,
	)
	return ck
}

// newK8sClient returns a new kubernetes client
func (ckFC *ChaoskubeConfig) newK8sClient() (*kubernetes.Clientset, error) {
	if ckFC.Kubeconfig == "" {
		if _, err := os.Stat(clientcmd.RecommendedHomeFile); err == nil {
			ckFC.Kubeconfig = clientcmd.RecommendedHomeFile
		}
	}

	log.WithFields(log.Fields{
		"kubeconfig": ckFC.Kubeconfig,
		"master":     ckFC.Master,
	}).Debug("using cluster config")

	config, err := clientcmd.BuildConfigFromFlags(ckFC.Master, ckFC.Kubeconfig)
	if err != nil {
		return nil, err
	}

	client, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, err
	}

	serverVersion, err := client.Discovery().ServerVersion()
	if err != nil {
		return nil, err
	}

	log.WithFields(log.Fields{
		"master":        config.Host,
		"serverVersion": serverVersion,
	}).Info("connected to cluster")

	return client, nil
}

func parseSelector(str string) labels.Selector {
	selector, err := labels.Parse(str)
	if err != nil {
		log.WithFields(log.Fields{
			"selector": str,
			"err":      err,
		}).Fatal("failed to parse selector")
	}
	return selector
}

func formatDays(days []time.Time) []string {
	formattedDays := make([]string, 0, len(days))
	for _, d := range days {
		formattedDays = append(formattedDays, d.Format(util.YearDay))
	}
	return formattedDays
}
