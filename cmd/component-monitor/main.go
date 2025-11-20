package main

import (
	"context"
	"os"
	"time"

	routeclientset "github.com/openshift/client-go/route/clientset/versioned/typed/route/v1"
	"github.com/prometheus/client_golang/api"
	v1 "github.com/prometheus/client_golang/api/prometheus/v1"
	"github.com/prometheus/common/model"
	"github.com/sirupsen/logrus"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/transport"
)

// PrometheusClient wraps the Prometheus API client for querying metrics.
type PrometheusClient struct {
	api v1.API
}

// NewPrometheusClient creates a new Prometheus client configured for OpenShift monitoring.
func NewPrometheusClient(_ string) (*PrometheusClient, error) {
	kubeconfigPath := os.Getenv("KUBECONFIG")
	if kubeconfigPath == "" {
		kubeconfigPath = "/etc/kubeconfig/config"
	}

	config, err := clientcmd.BuildConfigFromFlags("", kubeconfigPath)
	if err != nil {
		return nil, err
	}

	routeClient, err := routeclientset.NewForConfig(config)
	if err != nil {
		return nil, err
	}

	route, err := routeClient.Routes("openshift-monitoring").Get(context.Background(), "prometheus-k8s", metav1.GetOptions{})
	if err != nil {
		return nil, err
	}

	var addr string
	if route.Spec.TLS != nil {
		addr = "https://" + route.Spec.Host
	} else {
		addr = "http://" + route.Spec.Host
	}

	client, err := api.NewClient(api.Config{
		Address:      addr,
		RoundTripper: transport.NewBearerAuthRoundTripper(config.BearerToken, api.DefaultRoundTripper),
	})
	if err != nil {
		return nil, err
	}

	return &PrometheusClient{
		api: v1.NewAPI(client),
	}, nil
}

// QueryMetrics executes a list of Prometheus queries and logs the results.
func (p *PrometheusClient) QueryMetrics(ctx context.Context, queries []string) {
	for _, query := range queries {
		logrus.Infof("Executing query: %s", query)

		result, warnings, err := p.api.Query(ctx, query, time.Now())
		if err != nil {
			logrus.Errorf("Query failed: %v", err)
			continue
		}

		if len(warnings) > 0 {
			logrus.Warnf("Query warnings: %v", warnings)
		}

		switch v := result.(type) {
		case model.Vector:
			logrus.Infof("Vector result: %d samples", len(v))
			for _, sample := range v {
				logrus.Infof("Sample: %s = %f", sample.Metric, float64(sample.Value))
			}
		case *model.Scalar:
			logrus.Infof("Scalar result: %f", float64(v.Value))
		case model.Matrix:
			logrus.Infof("Matrix result: %d series", len(v))
			for _, series := range v {
				logrus.Infof("Series: %s (%d points)", series.Metric, len(series.Values))
			}
		default:
			logrus.Infof("Unknown result type: %T", result)
		}
	}
}

func main() {
	logrus.SetLevel(logrus.InfoLevel)
	logrus.SetFormatter(&logrus.TextFormatter{
		FullTimestamp: true,
	})

	prometheusURL := os.Getenv("PROMETHEUS_URL")
	if prometheusURL == "" {
		prometheusURL = "http://localhost:9090"
	}

	client, err := NewPrometheusClient(prometheusURL)
	if err != nil {
		logrus.WithFields(logrus.Fields{
			"prometheus_url": prometheusURL,
			"error":          err,
		}).Fatal("Failed to create Prometheus client")
	}

	queries := []string{
		"1",
		"absent(up{job=\"deck\"} == 1)",
		"absent(up{job=\"crier\"} == 1)",
	}

	ctx := context.Background()

	logrus.Info("Starting Prometheus metrics monitoring...")

	for {
		logrus.Info("Executing Prometheus queries...")
		client.QueryMetrics(ctx, queries)

		logrus.Info("Sleeping for 30 seconds...")
		time.Sleep(30 * time.Second)
	}
}
