package proxy

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"strconv"

	"blazar/internal/pkg/log"
)

type Proxy struct {
}

func NewProxy() *Proxy {
	return &Proxy{}
}

func (p *Proxy) ListenAndServe(ctx context.Context, cfg *Config) error {
	logger := log.FromContext(ctx)
	httpAddr := net.JoinHostPort(cfg.Host, strconv.Itoa(int(cfg.HTTPPort)))

	// register handlers
	http.HandleFunc("/", IndexHandler(cfg))

	logger.Infof("serving http server on %s", httpAddr)
	if err := http.ListenAndServe(httpAddr, nil); err != nil {
		fmt.Println("error serving http server", err)
		panic(err)
	}

	return nil
}
