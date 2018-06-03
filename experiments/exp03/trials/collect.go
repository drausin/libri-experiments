package main

import (
	"fmt"
	"log"
	"net/http"

	"github.com/spf13/cobra"
)

const timeWindow = "30m"

var (
	bytesStoredQueries = map[string]string{
		"bytes.stored.peer":        `sum by (pod_name)(grpc_server_doc_stored_size{})`,
		"bytes.stored.cluster":     `sum by ()(grpc_server_doc_stored_size{})`,
		"bytes.store-rate.peer":    fmt.Sprintf(`sum by (pod_name)(rate( grpc_server_doc_stored_size{}[%s]))`, timeWindow),
		"bytes.store-rate.cluster": fmt.Sprintf(`sum by ()(rate( grpc_server_doc_stored_size{}[%s]))`, timeWindow),
	}

	docsStoredQueries = map[string]string{
		"docs.stored.peer":        `sum by (pod_name)(grpc_server_doc_stored_count{})`,
		"docs.stored.cluster":     `sum by ()(grpc_server_doc_stored_count{})`,
		"docs.store-rate.peer":    fmt.Sprintf(`sum by (pod_name)(rate( grpc_server_doc_stored_count{}[%s]))`, timeWindow),
		"docs.store-rate.cluster": fmt.Sprintf(`sum by ()(rate( grpc_server_doc_stored_count{}[%s]))`, timeWindow),
	}
	methods        = []string{"Put", "Get"}
	quantiles      = []float64{0.5, 0.95}
	latencyQueries = getLatencyQueries(quantiles, methods)
	qpsQueries     = getQPSQueries(methods)

	grafanaPodName = ""
	outDir         = ""
	queryRangeURL  = "http://prometheus.default.svc.cluster.local:9090/api/v1/query"
	cmd            = &cobra.Command{
		Use: "collect",
		Run: func(cmd *cobra.Command, args []string) {
			queries := make(map[string]string)
			appendQueries(queries, bytesStoredQueries)
			appendQueries(queries, docsStoredQueries)
			appendQueries(queries, latencyQueries)
			appendQueries(queries, qpsQueries)

			fmt.Printf("mkdir -p %s\n", outDir)
			for label, query := range queries {
				rq, err := http.NewRequest(http.MethodGet, queryRangeURL, nil)
				if err != nil {
					log.Fatal(err)
				}
				q := rq.URL.Query()
				q.Add("query", query)
				fmt.Printf("kubectl exec %s -- curl '%s?%s' | gzip > %s/%s.json.gz\n",
					grafanaPodName, rq.URL.String(), q.Encode(), outDir, label)
			}
		},
	}
)

func init() {
	cmd.Flags().StringVar(&grafanaPodName, "grafanaPod", "", "Grafana pod name")
	cmd.Flags().StringVar(&outDir, "outDir", "", "output directory")
}

func main() {
	cmd.Execute()
}

func appendQueries(a, b map[string]string) {
	for label, query := range b {
		a[label] = query
	}
}

func getLatencyQueries(quantiles []float64, methods []string) map[string]string {
	qs := make(map[string]string)
	for _, quantile := range quantiles {
		for _, method := range methods {
			peerName := fmt.Sprintf("%s.p%d.peer", method, int(quantile*100))
			qs[peerName] = fmt.Sprintf(`histogram_quantile(%f, 
sum(rate(grpc_server_handling_seconds_bucket{grpc_type="unary",grpc_method="%s"}[%s])) by (pod_name, le)) * 1000`, quantile, method, timeWindow)
			clusterName := fmt.Sprintf("%s.p%d.cluster", method, int(quantile*100))
			qs[clusterName] = fmt.Sprintf(`histogram_quantile(%f, 
sum(rate(grpc_server_handling_seconds_bucket{grpc_type="unary",grpc_method="%s"}[%s])) by (le)) * 1000`, quantile, method, timeWindow)
		}
	}
	return qs
}

func getQPSQueries(methods []string) map[string]string {
	qs := make(map[string]string)
	qs["all.qps.peer"] = fmt.Sprintf(`sum by (pod_name)(rate( grpc_server_handled_total{}[%s] ))`, timeWindow)
	qs["all.qps.cluster"] = fmt.Sprintf(`sum by ()(rate( grpc_server_handled_total{}[%s] ))`, timeWindow)
	for _, method := range methods {
		peerName := fmt.Sprintf("%s.qps.peer", method)
		qs[peerName] = fmt.Sprintf(`sum by (pod_name)(rate( grpc_server_handled_total{grpc_method="%s"}[%s] ))`, method, timeWindow)
		allName := fmt.Sprintf("%s.qps.cluster", method)
		qs[allName] = fmt.Sprintf(`sum by ()(rate( grpc_server_handled_total{grpc_method="%s"}[%s] ))`, method, timeWindow)
	}
	return qs
}
