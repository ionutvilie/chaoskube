package notifier

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"strings"
	"time"
)

// TryDeliverReport ...
func TryDeliverReport(grafanaURL string, notifWebhook string, lockURL string, snapURL string) {
	// DRP_CF_VERTICAL
	// DRP_CF_LOCATION
	// DRP_CF_STAGE

	metaPayloadRaw := map[string]map[string]interface{}{
		"meta": {
			"name":      os.Getenv("DRP_CF_VERTICAL"),
			"dc":        os.Getenv("DRP_CF_LOCATION"),
			"env":       os.Getenv("DRP_CF_STAGE"),
			"timestamp": time.Now(),
		},
	}
	metaPayload, _ := json.Marshal(metaPayloadRaw)

	if storeMeta(lockURL, metaPayload) {
		grafanaSnapshotURL := getGrafanaSnapshot(grafanaURL, snapURL)
		deliverNotification(notifWebhook, grafanaSnapshotURL, metaPayloadRaw["meta"])
	}
}

func send(url string, meth string, data []byte) *http.Response {
	req, err := http.NewRequest(meth, url, bytes.NewBuffer(data))
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		panic(err)
	}
	return resp
}

func storeMeta(url string, data []byte) bool {
	// url := "https://reliability.metrosystems.net/staticapi/api/v1/chaoslock/store"
	resp := send(url, "PUT", data)
	defer resp.Body.Close()

	if strings.Compare(resp.Status, "201") == 0 {
		return true
	}
	return false
}

func getGrafanaSnapshot(dashURL string, snapURL string) string {
	resp, _ := http.Get(dashURL)
	// resp, _ := http.Get("https://reliability.metrosystems.net/performance/api/dashboards/uid/J43HxwzWz")

	payload := map[string]interface{}{
		"expires":   86400,
		"name":      "Snapshot for chaoskube run today",
		"dashboard": map[string]interface{}{},
	}
	defer resp.Body.Close()
	dashRaw := map[string]interface{}{}
	dashData, _ := ioutil.ReadAll(resp.Body)
	json.Unmarshal(dashData, &dashRaw)
	dash := dashRaw["dashboard"].(map[string]interface{})

	delete(dash, "uid")
	dash["title"] = "chaosRUN dash"
	dash["editable"] = false
	payload["dashboard"] = dash

	fmt.Println(payload)

	encPayload, _ := json.Marshal(payload)
	snapResponse := send(snapURL, "POST", encPayload)

	body, _ := ioutil.ReadAll(snapResponse.Body)
	defer snapResponse.Body.Close()

	snapData := map[string]string{}
	json.Unmarshal(body, &snapData)

	return snapData["url"]
}

func buildNotificationMessage(params map[string]interface{}, snapMessage string) (bytesData []byte) {
	ts, _ := time.Parse(time.RFC1123, params["timestamp"].(string))
	body := map[string]interface{}{
		"@type":      "MessageCard",
		"@context":   "https://schema.org/extensions",
		"summary":    "Some changes",
		"themeColor": "0078D7",
		"title":      "Reliability Checklist update",
		"sections": []map[string]interface{}{
			{
				"activityTitle":    fmt.Sprintf("ChaosKube report for %s", params["name"]),
				"activitySubtitle": ts.Format("2006-01-02 15:04:05"),
				"facts": []map[string]interface{}{
					{
						"name":  "Vertical Name:",
						"value": params["name"],
					},
					{
						"name":  "Datacenter:",
						"value": params["dc"],
					},
					{
						"name":  "Environment:",
						"value": params["env"],
					},
				},
				"text": snapMessage,
			},
		},
	}
	bytesData, _ = json.Marshal(body)
	return
}

func deliverNotification(webhook string, snapshotURL string, metadata map[string]interface{}) {
	snapMessage := fmt.Sprintf("Grafana snapshoot url: %s", snapshotURL)
	data := buildNotificationMessage(metadata, snapMessage)
	resp := send(webhook, "POST", data)
	defer resp.Body.Close()
}
