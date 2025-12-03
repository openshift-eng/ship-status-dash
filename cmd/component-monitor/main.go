package main

import (
	"context"
	"errors"
	"flag"
	"os"
	"os/signal"
	"syscall"
	"time"

	routeclientset "github.com/openshift/client-go/route/clientset/versioned/typed/route/v1"
	"github.com/prometheus/client_golang/api"
	v1 "github.com/prometheus/client_golang/api/prometheus/v1"
	"github.com/prometheus/common/model"
	"github.com/sirupsen/logrus"
	"gopkg.in/yaml.v3"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/transport"

	"ship-status-dash/pkg/types"
)

// PrometheusClient wraps the Prometheus API client for querying metrics.
type PrometheusClient struct {
	api v1.API
}

// Options contains command-line configuration options for the component monitor.
type Options struct {
	ConfigPath   string
	DashboardURL string
	Name         string
}

// NewOptions parses command-line flags and returns a new Options instance.
func NewOptions() *Options {
	opts := &Options{}

	flag.StringVar(&opts.ConfigPath, "config-path", "", "Path to component monitor config file")
	flag.StringVar(&opts.DashboardURL, "dashboard-url", "http://localhost:8080", "Dashboard API base URL")
	flag.StringVar(&opts.Name, "name", "", "Name of the component monitor")
	flag.Parse()

	return opts
}

// Validate checks that all required options are provided and valid.
func (o *Options) Validate() error {
	if o.ConfigPath == "" {
		return errors.New("config path is required (use --config-path flag)")
	}

	if _, err := os.Stat(o.ConfigPath); os.IsNotExist(err) {
		return errors.New("config file does not exist: " + o.ConfigPath)
	}

	if o.Name == "" {
		return errors.New("name is required (use --name flag)")
	}

	return nil
}

func loadAndValidateComponentsAndFrequency(log *logrus.Logger, configPath string) ([]types.MonitoringComponent, time.Duration) {
	log.Infof("Loading config from %s", configPath)

	configFile, err := os.ReadFile(configPath)
	if err != nil {
		log.WithFields(logrus.Fields{
			"config_path": configPath,
			"error":       err,
		}).Fatal("Failed to read config file")
	}

	var config types.ComponentMonitorConfig
	if err := yaml.Unmarshal(configFile, &config); err != nil {
		log.WithFields(logrus.Fields{
			"config_path": configPath,
			"error":       err,
		}).Fatal("Failed to parse config file")
	}

	frequency, err := time.ParseDuration(config.Frequency)
	if err != nil {
		log.WithFields(logrus.Fields{
			"frequency": config.Frequency,
			"error":     err,
		}).Fatal("Failed to parse frequency")
	}
	log.Infof("Probing Frequency configured to: %s", frequency)

	for _, component := range config.Components {
		if component.HTTPMonitor != nil {
			retryAfter, err := time.ParseDuration(component.HTTPMonitor.RetryAfter)
			if err != nil {
				log.WithField("error", err).Fatal("Failed to parse retry after duration")
			}
			if retryAfter > frequency {
				log.WithFields(logrus.Fields{
					"component":     component.ComponentSlug,
					"sub_component": component.SubComponentSlug,
					"retry_after":   component.HTTPMonitor.RetryAfter,
					"frequency":     frequency,
				}).Fatal("Retry after duration is greater than frequency")
			}
		}
	}

	log.Infof("Loaded configuration with %d components", len(config.Components))
	return config.Components, frequency
}

// NewPrometheusClient creates a new Prometheus client configured for OpenShift monitoring.
func NewPrometheusClient() (*PrometheusClient, error) {
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
	log := logrus.New()
	log.SetLevel(logrus.InfoLevel)
	log.SetFormatter(&logrus.TextFormatter{
		FullTimestamp: true,
	})

	opts := NewOptions()

	if err := opts.Validate(); err != nil {
		log.WithField("error", err).Fatal("Invalid command-line options")
	}

	components, frequency := loadAndValidateComponentsAndFrequency(log, opts.ConfigPath)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-sigChan
		log.Info("Received interrupt signal, shutting down...")
		cancel()
	}()

	probers := []*HTTPProber{}
	for _, component := range components {
		componentLogger := log.WithFields(logrus.Fields{
			"component":     component.ComponentSlug,
			"sub_component": component.SubComponentSlug,
		})
		componentLogger.Info("Configuring component monitor probe")
		if component.HTTPMonitor != nil {
			retryAfter, err := time.ParseDuration(component.HTTPMonitor.RetryAfter)
			if err != nil {
				componentLogger.WithField("error", err).Fatal("Failed to parse retry after duration")
			}
			prober := NewHTTPProber(component.ComponentSlug, component.SubComponentSlug, component.HTTPMonitor.URL, component.HTTPMonitor.Code, retryAfter)
			componentLogger.Info("Added HTTP prober for component")
			probers = append(probers, prober)
		}
	}

	if len(probers) == 0 {
		log.Warn("No probers configured, exiting")
		return
	}

	orchestrator := NewProbeOrchestrator(probers, frequency, opts.DashboardURL, opts.Name, log)
	orchestrator.Run(ctx)
}
