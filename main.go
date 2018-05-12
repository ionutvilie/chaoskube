package main

import (
	"encoding/json"
	"math/rand"
	"net/http"
	"os"
	"time"

	log "github.com/sirupsen/logrus"
	"gopkg.in/alecthomas/kingpin.v2"

	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/kubernetes"
	_ "k8s.io/client-go/plugin/pkg/client/auth"
	"k8s.io/client-go/tools/clientcmd"

	"github.com/ionutvilie/chaoskube/chaoskube"
	"github.com/ionutvilie/chaoskube/util"
)

var (
	version = "undefined"
	ckFC    = &chaoskubeFlagsConfig{}
)

var (
// ckFC *chaoskubeFlagsConfig
)

// flags should be bind to this structure
// flags should be falidated by a new method
// main function should be shrinked down
type chaoskubeFlagsConfig struct {
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

func init() {
	rand.Seed(time.Now().UTC().UnixNano())
	kingpin.Flag("labels", "A set of labels to restrict the list of affected pods. Defaults to everything.").StringVar(&ckFC.Labels)
	kingpin.Flag("annotations", "A set of annotations to restrict the list of affected pods. Defaults to everything.").StringVar(&ckFC.Annotations)
	kingpin.Flag("namespaces", "A set of namespaces to restrict the list of affected pods. Defaults to everything.").StringVar(&ckFC.Namespaces)
	kingpin.Flag("excluded-weekdays", "A list of weekdays when termination is suspended, e.g. Sat,Sun").StringVar(&ckFC.ExcludedWeekdays)
	kingpin.Flag("excluded-times-of-day", "A list of time periods of a day when termination is suspended, e.g. 22:00-08:00").StringVar(&ckFC.ExcludedTimesOfDay)
	kingpin.Flag("excluded-days-of-year", "A list of days of a year when termination is suspended, e.g. Apr1,Dec24").StringVar(&ckFC.ExcludedDaysOfYear)
	kingpin.Flag("timezone", "The timezone by which to interpret the excluded weekdays and times of day, e.g. UTC, Local, Europe/Berlin. Defaults to UTC.").Default("UTC").StringVar(&ckFC.Timezone)
	kingpin.Flag("master", "The address of the Kubernetes cluster to target").StringVar(&ckFC.Master)
	kingpin.Flag("kubeconfig", "Path to a kubeconfig file").StringVar(&ckFC.Kubeconfig)
	kingpin.Flag("interval", "Interval between Pod terminations").Default("10m").DurationVar(&ckFC.Interval)
	kingpin.Flag("dry-run", "If true, don't actually do anything.").Default("true").BoolVar(&ckFC.DryRun)
	kingpin.Flag("debug", "Enable debug logging.").BoolVar(&ckFC.Debug)
	kingpin.Flag("httpServer", "Enable httpServer.").Default("true").BoolVar(&ckFC.HTTPServer)
}

func main() {
	kingpin.Version(version)
	kingpin.Parse()

	if ckFC.Debug {
		log.SetLevel(log.DebugLevel)
	}

	log.WithFields(log.Fields{
		"labels":             ckFC.Labels,
		"annotations":        ckFC.Annotations,
		"namespaces":         ckFC.Namespaces,
		"excludedWeekdays":   ckFC.ExcludedWeekdays,
		"excludedTimesOfDay": ckFC.ExcludedTimesOfDay,
		"excludedDaysOfYear": ckFC.ExcludedDaysOfYear,
		"timezone":           ckFC.Timezone,
		"master":             ckFC.Master,
		"kubeconfig":         ckFC.Kubeconfig,
		"interval":           ckFC.Interval,
		"dryRun":             ckFC.DryRun,
		"debug":              ckFC.Debug,
	}).Debug("reading config")

	log.WithFields(log.Fields{
		"version":  version,
		"dryRun":   ckFC.DryRun,
		"interval": ckFC.Interval,
	}).Info("starting up")

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

	chaoskube := chaoskube.New(
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

	if ckFC.HTTPServer {
		go httpMuxServer()
	}

	for {
		if err := chaoskube.TerminateVictim(); err != nil {
			log.WithField("err", err).Error("failed to terminate victim")
		}

		log.WithField("duration", ckFC.Interval).Debug("sleeping")
		time.Sleep(ckFC.Interval)
	}

}

// newK8sClient returns a new kubernetes client
func (ckFC *chaoskubeFlagsConfig) newK8sClient() (*kubernetes.Clientset, error) {
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

// simple httpServer
func httpMuxServer() {

	mux := http.NewServeMux()
	mux.HandleFunc("/api/v1/config", configHandler)     //
	mux.HandleFunc("/.well-known/live", healthHandler)  // k8s pod process started
	mux.HandleFunc("/.well-known/ready", healthHandler) // k8s pod is ready to accept traffic

	// log.WithFields("info", "http server").Info("http server started on :8080")
	log.Infoln("http server started on :8080")
	err := http.ListenAndServe(":8080", mux)
	if err != nil {
		log.WithField("err", err).Fatal("ListenAndServe")
	}

}

// healthHandler to be used for k8s live and ready probe
func healthHandler(wr http.ResponseWriter, req *http.Request) {
	wr.WriteHeader(http.StatusOK)
	wr.Write([]byte(`{"Status": OK}`))
}

// configHandler manages chaoskube configuration
// method get  --> gets monkey config
// method post --> updates config // -- not implemented
func configHandler(wr http.ResponseWriter, req *http.Request) {
	wr.Header().Set("Access-Control-Allow-Origin", "*")
	configData, err := json.Marshal(ckFC)
	if err != nil {
		http.Error(wr, err.Error(), http.StatusInternalServerError)
		return
	}
	wr.Header().Set("Content-Type", "application/json")
	wr.Write(configData)
}
