package main

import (
	"encoding/json"
	"math/rand"
	"net/http"
	"time"

	log "github.com/sirupsen/logrus"
	"gopkg.in/alecthomas/kingpin.v2"

	_ "k8s.io/client-go/plugin/pkg/client/auth"

	"github.com/metrosystems-cpe/chaoskube/internal"
)

var (
	version = "undefined"
	ckConf  = &internal.ChaoskubeConfig{}
	quit    = make(chan bool) // channel used to send "kill" message to routine where monkey run.
)

func init() {
	rand.Seed(time.Now().UTC().UnixNano())

	kingpin.Flag("labels", "A set of labels to restrict the list of affected pods. Defaults to everything.").StringVar(&ckConf.Labels)
	kingpin.Flag("annotations", "A set of annotations to restrict the list of affected pods. Defaults to everything.").StringVar(&ckConf.Annotations)
	kingpin.Flag("namespaces", "A set of namespaces to restrict the list of affected pods. Defaults to everything.").StringVar(&ckConf.Namespaces)
	kingpin.Flag("excluded-weekdays", "A list of weekdays when termination is suspended, e.g. Sat,Sun").StringVar(&ckConf.ExcludedWeekdays)
	kingpin.Flag("excluded-times-of-day", "A list of time periods of a day when termination is suspended, e.g. 22:00-08:00").StringVar(&ckConf.ExcludedTimesOfDay)
	kingpin.Flag("excluded-days-of-year", "A list of days of a year when termination is suspended, e.g. Apr1,Dec24").StringVar(&ckConf.ExcludedDaysOfYear)
	kingpin.Flag("timezone", "The timezone by which to interpret the excluded weekdays and times of day, e.g. UTC, Local, Europe/Berlin. Defaults to UTC.").Default("UTC").StringVar(&ckConf.Timezone)
	kingpin.Flag("master", "The address of the Kubernetes cluster to target").StringVar(&ckConf.Master)
	kingpin.Flag("kubeconfig", "Path to a kubeconfig file").StringVar(&ckConf.Kubeconfig)
	kingpin.Flag("interval", "Interval between Pod terminations").Default("10m").DurationVar(&ckConf.Interval)
	kingpin.Flag("dry-run", "If true, don't actually do anything.").Default("true").BoolVar(&ckConf.DryRun)
	kingpin.Flag("debug", "Enable debug logging.").BoolVar(&ckConf.Debug)
	kingpin.Flag("httpServer", "Enable httpServer.").Default("true").BoolVar(&ckConf.HTTPServer)
}

func main() {
	kingpin.Version(version)
	kingpin.Parse()

	if ckConf.Debug {
		log.SetLevel(log.DebugLevel)
	}

	go startMonkey()
	httpMuxServer()
}

// simple httpServer
func httpMuxServer() {

	mux := http.NewServeMux()
	mux.HandleFunc("/api/v1/config", configHandler)       //
	mux.HandleFunc("/.well-known/live", healthHandler)    // k8s pod process started
	mux.HandleFunc("/.well-known/ready", healthHandler)   // k8s pod is ready to accept traffic
	mux.HandleFunc("/api/v1/update", updateConfigHandler) // k8s pod is ready to accept traffic

	// log.WithFields("info", "http server").Info("http server started on :8080")
	log.Infoln("http server started on :8080")
	err := http.ListenAndServe(":8080", mux)
	if err != nil {
		log.WithField("err", err).Fatal("ListenAndServe")
	}
}

func updateConfigHandler(wr http.ResponseWriter, req *http.Request) {
	newConf := internal.ChaoskubeConfig{}
	decoder := json.NewDecoder(req.Body)
	err := decoder.Decode(&newConf)
	// TODO: Need to find a way to validate config :-?

	if err != nil {
		log.Infof("Fail to decode params. Error: %v", err)
		wr.WriteHeader(http.StatusInternalServerError)
		wr.Write([]byte(`{"Status": "Something went wrong. Check logs..."}`)) // Need better error message
	} else {
		log.Info("Config updated and will be used after monkey finishes sleep.")
		ckConf = newConf.Diff(ckConf)
		go func() {
			// kill old monkey by pushing true on quit channel
			quit <- true
			// restart monkey with new config
			go startMonkey()
		}()
		wr.WriteHeader(http.StatusOK)
		wr.Write([]byte(`{"Status": "Config will be used after monkey finishes sleeping period"}`)) // Need better error message

	}

}

func startMonkey() {
	log.WithFields(log.Fields{
		"version":  version,
		"dryRun":   ckConf.DryRun,
		"interval": ckConf.Interval,
	}).Info("Monkey start")

	monkey := ckConf.NewMonkey()

	for {
		select {
		case <-quit:
			return
		default:
			if err := monkey.TerminateVictim(); err != nil {
				log.WithField("err", err).Error("failed to terminate victim")
			}

			log.WithField("duration", ckConf.Interval).Debug("sleeping")
			time.Sleep(ckConf.Interval)
			// log.Infof("Killing stuff %v", ckConf.Interval)
			// time.Sleep(ckConf.Interval)
		}
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
	configData, err := json.Marshal(ckConf)
	if err != nil {
		http.Error(wr, err.Error(), http.StatusInternalServerError)
		return
	}
	wr.Header().Set("Content-Type", "application/json")
	wr.Write(configData)
}
