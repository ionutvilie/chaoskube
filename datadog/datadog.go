package datadog

import (
	"fmt"
	"os"

	"github.com/DataDog/datadog-go/statsd"
	log "github.com/sirupsen/logrus"
	v1 "k8s.io/api/core/v1"
)

// NewDDClient ...
func NewDDClient() (Client *statsd.Client) {

	agentHost := os.Getenv("DRP_CF_KUBERNETES_MINION_NAME")
	k8sNamespace := os.Getenv("DRP_CF_KUBERNETES_NAMESPACE")

	c, err := statsd.New(fmt.Sprintf("%s:8125", agentHost))
	if err != nil {
		log.Fatal(err)
	}
	c.Namespace = k8sNamespace
	return c
}

// NewEvent ...
func NewEvent(client *statsd.Client, victim v1.Pod) error {
	var e *statsd.Event
	e.AlertType = "info"
	e.Hostname, _ = os.Hostname()
	e.Title = "[ChaosKube] " + victim.Name + " was killed"
	e.Text = "Pod" + victim.Name + "was deleted by ChaosKube"
	e.Priority = "low"

	err := client.Event(e)
	if err != nil {
		log.Fatal(err)
	}
	return err
}
