package metrics

import (
	"net/http"

	checksproto "blazar/internal/pkg/proto/daemon"

	"github.com/grpc-ecosystem/grpc-gateway/v2/runtime"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

const (
	namespace      = "blazar"
	proxyNamespace = "blazar_proxy"
)

type Metrics struct {
	Up                 prometheus.Gauge
	BlocksToUpgrade    *prometheus.GaugeVec
	LastObservedHeight prometheus.Gauge
	UpwErrs            prometheus.Counter
	UiwErrs            prometheus.Counter
	HwErrs             prometheus.Counter
	NotifErrs          prometheus.Counter
}

func NewMetrics(composeFile, hostname, version, chainID string) *Metrics {
	labels := prometheus.Labels{"hostname": hostname, "compose_file": composeFile, "version": version, "chain_id": chainID}

	preChecks := make([]string, 0, len(checksproto.PreCheck_value))
	for pc := range checksproto.PreCheck_value {
		preChecks = append(preChecks, pc)
	}

	postChecks := make([]string, 0, len(checksproto.PostCheck_value))
	for pc := range checksproto.PostCheck_value {
		postChecks = append(postChecks, pc)
	}

	blocksToUpgradeLabels := []string{"upgrade_height", "upgrade_name", "upgrade_status", "upgrade_step", "validator_address", "upgrade_tag"}
	blocksToUpgradeLabels = append(blocksToUpgradeLabels, preChecks...)
	blocksToUpgradeLabels = append(blocksToUpgradeLabels, postChecks...)

	metrics := &Metrics{
		Up: promauto.NewGauge(
			prometheus.GaugeOpts{
				Namespace:   namespace,
				Name:        "up",
				Help:        "Is blazar up?",
				ConstLabels: labels,
			},
		),
		BlocksToUpgrade: promauto.NewGaugeVec(
			prometheus.GaugeOpts{
				Namespace:   namespace,
				Name:        "blocks_to_upgrade_height",
				Help:        "Number of blocks to the upgrade height",
				ConstLabels: labels,
			},
			blocksToUpgradeLabels,
		),
		LastObservedHeight: promauto.NewGauge(
			prometheus.GaugeOpts{
				Namespace:   namespace,
				Name:        "last_observed_height",
				Help:        "Last block height observed by the height watcher",
				ConstLabels: labels,
			},
		),
		UpwErrs: promauto.NewCounter(
			prometheus.CounterOpts{
				Namespace:   namespace,
				Name:        "upgrade_proposals_watcher_errors",
				Help:        "Upgrade proposals watcher error count",
				ConstLabels: labels,
			},
		),
		UiwErrs: promauto.NewCounter(
			prometheus.CounterOpts{
				Namespace:   namespace,
				Name:        "upgrade_info_watcher_errors",
				Help:        "upgrade-info.json watcher error count",
				ConstLabels: labels,
			},
		),
		HwErrs: promauto.NewCounter(
			prometheus.CounterOpts{
				Namespace:   namespace,
				Name:        "height_watcher_errors",
				Help:        "Chain height watcher error count",
				ConstLabels: labels,
			},
		),
		NotifErrs: promauto.NewCounter(
			prometheus.CounterOpts{
				Namespace:   namespace,
				Name:        "notifier_errors",
				Help:        "Notifier error count",
				ConstLabels: labels,
			},
		),
	}

	return metrics
}

type ProxyMetrics struct {
	ConnErrs *prometheus.CounterVec
}

func NewProxyMetrics() *ProxyMetrics {
	instanceLabels := []string{"name", "host", "http_port", "grpc_port", "network"}
	connectionErrors := promauto.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: proxyNamespace,
			Name:      "connection_errors",
			Help:      "Proxy connections error count",
		},
		instanceLabels,
	)
	return &ProxyMetrics{ConnErrs: connectionErrors}
}

func RegisterHandler(mux *runtime.ServeMux) error {
	handler := promhttp.Handler()
	return mux.HandlePath("GET", "/metrics", func(w http.ResponseWriter, r *http.Request, _ map[string]string) {
		handler.ServeHTTP(w, r)
	})
}
