package analysis

import (
	"net/http"
	"time"
	"log"
	"io/ioutil"
)

var queries = map[string]string{
	"get.p95": `histogram_quantile(0.95, sum(rate(grpc_server_handling_seconds_bucket{grpc_type="unary",grpc_method="Get"}[5m])) by (pod_name, le)) * 1000`
	"get.p50": `histogram_quantile(0.95, sum(rate(grpc_server_handling_seconds_bucket{grpc_type="unary",grpc_method="Get"}[5m])) by (pod_name, le)) * 1000`
	"peer_query_count": `libri_goodwill_peer_query_count{outcome="SUCCESS"}`,
}

func main() {
	queryRangeURL := "http://prometheus.default.svc.cluster.local:9090/api/v1/query_range"
	startTime := "2018-04-21T18:30:00Z"
	endTime := "2018-04-21T19:00:00Z"
	step := "1m"
	client := http.Client{Timeout: 20 * time.Second}

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

		rp, err := client.Do(rq)
		if err != nil {
			log.Fatal(err)
		}
		bodyBytes, err := ioutil.ReadAll(rp.Body)
		rp.Body.Close()
		if err != nil {
			log.Fatal(err)
		}
	}
}

