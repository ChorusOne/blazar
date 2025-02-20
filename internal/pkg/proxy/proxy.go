package proxy

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"strconv"

	"blazar/internal/pkg/log"
	"blazar/internal/pkg/metrics"

	"github.com/prometheus/client_golang/prometheus/promhttp"
)

type Proxy struct {
}

func NewProxy() *Proxy {
	return &Proxy{}
}

func (p *Proxy) ListenAndServe(ctx context.Context, cfg *Config) error {
	logger := log.FromContext(ctx)
	httpAddr := net.JoinHostPort(cfg.Host, strconv.Itoa(int(cfg.HTTPPort)))

	mux := http.NewServeMux()

	// register handlers
	proxyMetrics := metrics.NewProxyMetrics()
	mux.HandleFunc("/", IndexHandler(cfg, proxyMetrics))
	mux.Handle("/metrics", promhttp.Handler())

	server := &http.Server{
		Addr:    httpAddr,
		Handler: mux,
	}

	logger.Infof("serving http server on %s", httpAddr)
	if err := server.ListenAndServe(); err != nil {
		fmt.Println("error serving http server", err)
		panic(err)
	}

	return nil
}
