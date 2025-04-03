package proxy

import (
	"blazar/internal/pkg/metrics"
	"context"
	"encoding/base64"
	"fmt"
	"net"
	"net/http"
	"sort"
	"strconv"
	"sync"
	"text/template"
	"time"

	urproto "blazar/internal/pkg/proto/upgrades_registry"
	"blazar/internal/pkg/static"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

type instancePair struct {
	LastUpgrade *urproto.Upgrade
	Instance    Instance
	Error       error
}

func IndexHandler(cfg *Config, proxyMetrics *metrics.ProxyMetrics) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {

		start := time.Now()
		networkUpgrades := CheckInstances(r.Context(), cfg, proxyMetrics)
		end := time.Now()

		noInstances, noActive, noExecuting, noExpired, noCompleted, noErrors := uint(0), uint(0), uint(0), uint(0), uint(0), uint(0)
		for network, pairs := range networkUpgrades {
			noInstances += uint(len(pairs))

			for _, pair := range pairs {
				if pair.LastUpgrade != nil && pair.LastUpgrade.Status == urproto.UpgradeStatus_ACTIVE {
					noActive++
				}

				if pair.LastUpgrade != nil && pair.LastUpgrade.Status == urproto.UpgradeStatus_EXECUTING {
					noExecuting++
				}

				if pair.LastUpgrade != nil && pair.LastUpgrade.Status == urproto.UpgradeStatus_EXPIRED {
					noExpired++
				}

				if pair.LastUpgrade != nil && pair.LastUpgrade.Status == urproto.UpgradeStatus_COMPLETED {
					noCompleted++
				}

				if pair.Error != nil {
					proxyMetrics.ConnErrs.WithLabelValues(pair.Instance.Name,
						pair.Instance.Host, strconv.Itoa(pair.Instance.HTTPPort), strconv.Itoa(pair.Instance.GRPCPort), pair.Instance.Network).Inc()
					noErrors++
				}
			}

			sort.Slice(networkUpgrades[network], func(i, j int) bool {
				return networkUpgrades[network][i].Instance.Name > networkUpgrades[network][j].Instance.Name
			})
		}

		t, err := template.ParseFS(static.Templates, "templates/index/index-proxy.html")
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			_, _ = w.Write([]byte(err.Error()))
			return
		}

		logoData, err := static.Templates.ReadFile("templates/index/logo.png")
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			_, _ = w.Write([]byte(err.Error()))
			return
		}

		warning := ""
		if noErrors > 0 {
			warning = fmt.Sprintf("Encountered issues with %d instances. Some may be unreachable or failed completly, please investigate error messages", noErrors)
		}

		err = t.Execute(w, struct {
			NoNetworks  uint
			NoInstances uint
			NoActive    uint
			NoExecuting uint
			NoExpired   uint
			NoCompleted uint
			NoErrors    uint
			Upgrades    map[string][]instancePair
			FetchTime   time.Duration
			LogoBase64  string
			Warning     string
		}{
			NoNetworks:  uint(len(networkUpgrades)),
			NoInstances: noInstances,
			NoActive:    noActive,
			NoExecuting: noExecuting,
			NoExpired:   noExpired,
			NoCompleted: noCompleted,
			NoErrors:    noErrors,
			Upgrades:    networkUpgrades,
			FetchTime:   end.Sub(start),
			LogoBase64:  base64.StdEncoding.EncodeToString(logoData),
			Warning:     warning,
		})
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			_, _ = w.Write([]byte(err.Error()))
			return
		}
	}
}

func CheckInstances(_ context.Context, cfg *Config, proxyMetrics *metrics.ProxyMetrics) map[string][]instancePair {
	var (
		mutex           sync.Mutex
		networkUpgrades = make(map[string][]instancePair)
		wg              sync.WaitGroup
	)

	for _, instance := range cfg.Instances {
		wg.Add(1)

		if _, ok := networkUpgrades[instance.Network]; !ok {
			networkUpgrades[instance.Network] = []instancePair{}
		}

		go func(instance Instance) {
			defer wg.Done()

			withError := func(err error) {
				mutex.Lock()
				defer mutex.Unlock()

				networkUpgrades[instance.Network] = append(
					networkUpgrades[instance.Network],
					instancePair{
						LastUpgrade: nil,
						Instance:    instance,
						Error:       err,
					},
				)

				proxyMetrics.ConnErrs.WithLabelValues(
					instance.Name, instance.Host,
					strconv.Itoa(instance.HTTPPort),
					strconv.Itoa(instance.GRPCPort),
					instance.Network,
				).Inc()
			}

			address := net.JoinHostPort(instance.Host, strconv.Itoa(instance.GRPCPort))
			conn, err := grpc.NewClient(address, grpc.WithTransportCredentials(insecure.NewCredentials()))
			if err != nil {
				withError(err)
				return
			}
			defer conn.Close()

			c := urproto.NewUpgradeRegistryClient(conn)
			limit := int64(1)
			resp, err := c.ListUpgrades(context.Background(), &urproto.ListUpgradesRequest{
				DisableCache: false,
				Limit:        &limit,
			})
			if err != nil {
				withError(err)
				return
			}

			var lastUpgrade *urproto.Upgrade
			if len(resp.Upgrades) > 0 {
				lastUpgrade = resp.Upgrades[0]
			}

			if lastUpgrade != nil && lastUpgrade.Network != instance.Network {
				withError(fmt.Errorf("instance %s returned upgrade for network %s, expected %s",
					instance.Host, lastUpgrade.Network, instance.Network,
				))
				return
			}

			mutex.Lock()
			defer mutex.Unlock()

			networkUpgrades[instance.Network] = append(networkUpgrades[instance.Network], instancePair{
				LastUpgrade: lastUpgrade,
				Instance:    instance,
				Error:       nil,
			})
		}(instance)
	}

	wg.Wait()
	return networkUpgrades
}
