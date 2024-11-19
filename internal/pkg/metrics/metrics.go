package metrics

import (
	"net/http"

	"github.com/grpc-ecosystem/grpc-gateway/v2/runtime"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

const (
	namespace = "blazar"
)

type Metrics struct {
	Up              prometheus.Gauge
	Step            *prometheus.GaugeVec
	BlocksToUpgrade *prometheus.GaugeVec
	UpwErrs         prometheus.Counter
	UiwErrs         prometheus.Counter
	HwErrs          prometheus.Counter
	NotifErrs       prometheus.Counter
}

func NewMetrics(composeFile string, hostname string, version string) (*Metrics, error) {
	labels := prometheus.Labels{"hostname": hostname, "compose_file": composeFile, "version": version}

	metrics := &Metrics{
		Up: promauto.NewGauge(
			prometheus.GaugeOpts{
				Namespace:   namespace,
				Name:        "up",
				Help:        "Is blazar up?",
				ConstLabels: labels,
			},
		),
		Step: promauto.NewGaugeVec(
			prometheus.GaugeOpts{
				Namespace:   namespace,
				Name:        "upgrade_step",
				Help:        "ID of the current step of the upgrade process",
				ConstLabels: labels,
			},
			[]string{"upgrade_height", "upgrade_name", "proposal_status"},
		),
		BlocksToUpgrade: promauto.NewGaugeVec(
			prometheus.GaugeOpts{
				Namespace:   namespace,
				Name:        "blocks_to_upgrade_height",
				Help:        "Number of blocks to the upgrade height",
				ConstLabels: labels,
			},
			[]string{"upgrade_height", "upgrade_name", "proposal_status"},
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

	return metrics, nil
}

func RegisterHandler(mux *runtime.ServeMux) error {
	handler := promhttp.Handler()
	return mux.HandlePath("GET", "/metrics", func(w http.ResponseWriter, r *http.Request, _ map[string]string) {
		handler.ServeHTTP(w, r)
	})
}
