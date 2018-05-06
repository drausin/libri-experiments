package main

import (
	"fmt"
	"log"
	"net/http"
)

var queries = map[string]string{
	"get.p95":          getQuantileQuery("Get", 0.95),
	"get.p50":          getQuantileQuery("Get", 0.50),
	"put.p95":          getQuantileQuery("Put", 0.95),
	"put.p50":          getQuantileQuery("Put", 0.50),
	"peer_query_count": `libri_goodwill_peer_query_count{outcome="SUCCESS"}`,
}

func main() {
	queryRangeURL := "http://prometheus.default.svc.cluster.local:9090/api/v1/query_range"
	startTime := "2018-05-05T23:15:00Z"
	endTime := "2018-05-06T00:15:00Z"
	step := "1m"
	podName := "grafana-5bcc55f46f-mn7pv"
	outDir := "../trial10/data"

	for label, query := range queries {
		rq, err := http.NewRequest(http.MethodGet, queryRangeURL, nil)
		if err != nil {
			log.Fatal(err)
		}
		q := rq.URL.Query()
		q.Add("query", query)
		q.Add("start", startTime)
		q.Add("end", endTime)
		q.Add("step", step)
		fmt.Printf("kubectl exec %s -- curl '%s?%s' | gzip > %s/%s.json.gz\n",
			podName, rq.URL.String(), q.Encode(), outDir, label)
	}
}

func getQuantileQuery(endpoint string, quantile float32) string {
	return fmt.Sprintf(
		`histogram_quantile(%f, sum(rate(grpc_server_handling_seconds_`+
			`bucket{grpc_type="unary",grpc_method="%s"}[5m])) by (pod_name, le)) * 1000`,
		quantile,
		endpoint,
	)
}
