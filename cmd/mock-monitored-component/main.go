package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"net/http"
	"sync"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

var (
	activeStatusCode int
	statusMutex      sync.RWMutex

	successRateMetric     prometheus.Gauge
	dataLoadFailureMetric *prometheus.GaugeVec
	requestCountMetric    prometheus.Gauge
)

func init() {
	activeStatusCode = 200

	successRateMetric = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "success_rate",
		Help: "Mock success rate metric (Scalar)",
	})
	successRateMetric.Set(1.0)

	dataLoadFailureMetric = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "data_load_failure",
			Help: "Mock data load failure metric with labels (Vector)",
		},
		[]string{"component"},
	)
	dataLoadFailureMetric.WithLabelValues("api").Set(0.0)

	requestCountMetric = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "request_count",
		Help: "Mock request count metric (Matrix)",
	})
	requestCountMetric.Set(0.0)

	prometheus.MustRegister(successRateMetric, dataLoadFailureMetric, requestCountMetric)
}

func healthHandler(w http.ResponseWriter, r *http.Request) {
	statusMutex.RLock()
	code := activeStatusCode
	statusMutex.RUnlock()

	w.WriteHeader(code)
	fmt.Fprintf(w, "Status: %d\n", code)
}

func upHandler(w http.ResponseWriter, r *http.Request) {
	statusMutex.Lock()
	activeStatusCode = 200
	statusMutex.Unlock()

	w.WriteHeader(http.StatusOK)
	fmt.Fprintln(w, "Status set to 200")
}

func downHandler(w http.ResponseWriter, r *http.Request) {
	statusMutex.Lock()
	activeStatusCode = 500
	statusMutex.Unlock()

	w.WriteHeader(http.StatusOK)
	fmt.Fprintln(w, "Status set to 500")
}

func updateMetricsHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		fmt.Fprintln(w, "Method not allowed")
		return
	}

	var req struct {
		SuccessRate     *float64            `json:"success_rate"`
		DataLoadFailure *map[string]float64 `json:"data_load_failure"`
		RequestCount    *float64            `json:"request_count"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		fmt.Fprintf(w, "Invalid JSON: %v\n", err)
		return
	}

	if req.SuccessRate != nil {
		successRateMetric.Set(*req.SuccessRate)
	}
	if req.DataLoadFailure != nil {
		for component, value := range *req.DataLoadFailure {
			dataLoadFailureMetric.WithLabelValues(component).Set(value)
		}
	}
	if req.RequestCount != nil {
		requestCountMetric.Set(*req.RequestCount)
	}

	w.WriteHeader(http.StatusOK)
	fmt.Fprintln(w, "Metrics updated")
}

func main() {
	port := flag.String("port", "8080", "Port to listen on")
	flag.Parse()

	http.HandleFunc("/health", healthHandler)
	http.HandleFunc("/up", upHandler)
	http.HandleFunc("/down", downHandler)
	http.HandleFunc("/update-metrics", updateMetricsHandler)
	http.Handle("/metrics", promhttp.Handler())

	addr := ":" + *port
	fmt.Printf("Mock HTTP server starting on %s\n", addr)
	if err := http.ListenAndServe(addr, nil); err != nil {
		fmt.Printf("Server failed: %v\n", err)
	}
}
