package internal

import (
	"os"
	"reflect"
	"time"

	log "github.com/sirupsen/logrus"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"

	"github.com/metrosystems-cpe/chaoskube/chaoskube"
	"github.com/metrosystems-cpe/chaoskube/util"
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

// Diff method used to update config after api call
func (newConfig *ChaoskubeConfig) Diff(oldConfig *ChaoskubeConfig) *ChaoskubeConfig {
	v := reflect.ValueOf(*oldConfig)
	oldConfigStruct := reflect.ValueOf(oldConfig).Elem()
	newConfigStruct := reflect.ValueOf(newConfig).Elem()

	for i := 0; i < v.NumField(); i++ {
		fieldName := v.Type().Field(i).Name
		oldFieldValue := oldConfigStruct.FieldByName(fieldName)
		newFieldValue := newConfigStruct.FieldByName(fieldName)

		interfaceVal := newFieldValue.Interface()
		switch tp := newFieldValue.Kind(); tp {
		case reflect.String:
			val := interfaceVal.(string)
			if val != "" && val != oldFieldValue.Interface().(string) {
				structField := oldConfigStruct.FieldByName(fieldName)
				structField.Set(newFieldValue)
			}
		case reflect.Bool:
			val := interfaceVal.(bool)
			if val != oldFieldValue.Interface().(bool) {
				structField := oldConfigStruct.FieldByName(fieldName)
				structField.Set(newFieldValue)
			}
		default:
			val := interfaceVal.(time.Duration)
			if val != oldFieldValue.Interface().(time.Duration) {
				structField := oldConfigStruct.FieldByName(fieldName)
				structField.Set(newFieldValue)
			}
		}
	}

	result := oldConfigStruct.Interface().(ChaoskubeConfig)
	return &result
}

func (ckFC *ChaoskubeConfig) NewMonkey() *chaoskube.Chaoskube {

	client, err := ckFC.newK8sClient()
	if err != nil {
		log.Debugf("Failed to connect to cluster. %v", err)
	}

	var (
		labelSelector = parseSelector(ckFC.Labels)
		annotations   = parseSelector(ckFC.Annotations)
		namespaces    = parseSelector(ckFC.Namespaces)
	)

	log.Infof("Setting pod filters. Labels: [ %v ],  Annotations: [ %v ], Namespaces: [ %v ]", labelSelector, annotations, namespaces)

	parsedWeekdays := util.ParseWeekdays(ckFC.ExcludedWeekdays)
	parsedTimesOfDay, err := util.ParseTimePeriods(ckFC.ExcludedTimesOfDay)
	if err != nil {
		log.Fatalf("failed to parse times of day. timesOfDay: [ %v ], err: %v", ckFC.ExcludedTimesOfDay, err)
	}
	parsedDaysOfYear, err := util.ParseDays(ckFC.ExcludedDaysOfYear)
	if err != nil {
		log.Fatalf("failed to parse days of year. daysOfYear: [ %v ], err: %v", ckFC.ExcludedDaysOfYear, err)
	}

	log.Infof("Setting quiet times... Weeks: %v, timesOfDay: %v, daysOfYear: %v", parsedWeekdays, parsedTimesOfDay, formatDays(parsedDaysOfYear))

	parsedTimezone, err := time.LoadLocation(ckFC.Timezone)
	if err != nil {
		log.Fatalf("Failed to detect time zone. tz: %v, err: %v", ckFC.Timezone, err)
	}
	timezoneName, offset := time.Now().In(parsedTimezone).Zone()
	log.Infof("Setting timezone to: name: %s, location: %s, offset: %d", timezoneName, parsedTimezone, offset/int(time.Hour/time.Second))

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

	log.Infof("Connected to cluster. Server version: %s", serverVersion)

	return client, nil
}

func parseSelector(str string) labels.Selector {
	selector, err := labels.Parse(str)
	if err != nil {
		log.Fatal("failed to parse selector")
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
