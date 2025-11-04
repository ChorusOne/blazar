package daemon

import (
	"cmp"
	"encoding/base64"
	"fmt"
	"math"
	"net/http"
	"slices"
	"strconv"
	"strings"
	"text/template"
	"time"

	"blazar/internal/pkg/daemon/util"
	urproto "blazar/internal/pkg/proto/upgrades_registry"
	"blazar/internal/pkg/static"

	"github.com/grpc-ecosystem/grpc-gateway/v2/runtime"
)

func RegisterIndexHandler(mux *runtime.ServeMux, d *Daemon, upInterval time.Duration) error {
	return mux.HandlePath("GET", "/", func(w http.ResponseWriter, r *http.Request, _ map[string]string) {
		funcs := template.FuncMap{
			"formatTime": func(ts uint64) string {
				if ts == 0 {
					return "-"
				}
				return time.Unix(int64(ts), 0).Format("2006-01-02 15:04:05 MST")
			},
		}

		t, err := template.New("index-blazar.html").
			Funcs(funcs).
			ParseFS(static.Templates, "templates/index/index-blazar.html")
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		logoData, err := static.Templates.ReadFile("templates/index/logo.png")
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			_, _ = w.Write([]byte(err.Error()))
			return
		}

		disableCache := false
		value := r.FormValue("disable_cache")
		if value == "true" {
			disableCache = true
		}

		all, err := d.ur.GetAllUpgrades(r.Context(), !disableCache)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			_, _ = w.Write([]byte(err.Error()))
			return
		}

		warning := ""
		latestHeight, err := d.cosmosClient.GetLatestBlockHeight(r.Context())
		if err != nil {
			warning = "Failed to get latest block height from Cosmos: " + err.Error()
		}

		syncInfo := d.ur.SyncInfo()
		stateMachine := d.ur.GetStateMachine()

		blockSpeed := d.currBlockSpeed

		blocksToUpgradeMap := make(map[int64]string)
		blocksToETAMap := make(map[int64]string)
		upgrades, i := make([]*urproto.Upgrade, len(all)), 0

		for _, upgrade := range all {
			upgrade.Status = stateMachine.GetStatus(upgrade.Height)
			upgrade.Step = stateMachine.GetStep(upgrade.Height)

			blocksToUpgrade := ""
			if latestHeight != 0 {
				blocksToUpgrade = strconv.FormatInt(upgrade.GetHeight()-latestHeight, 10)
			}

			blocksToUpgradeMap[upgrade.Height] = blocksToUpgrade
			eta := time.Duration((upgrade.GetHeight()-latestHeight)*blockSpeed.Milliseconds()) * time.Millisecond
			blocksToETAMap[upgrade.Height] = formatRelativeTime(time.Now().Add(eta))
			upgrades[i] = upgrade
			i++
		}

		// sort descending by height, because we humans like to have the upcoming upgrades at the top
		slices.SortFunc(upgrades, func(i, j *urproto.Upgrade) int {
			return cmp.Compare(j.Height, i.Height)
		})

		if syncInfo.LastUpdateTime.IsZero() {
			warning = "Blazar haven't synced with the Cosmos network yet. Please wait for the first sync to complete."
		}

		err = t.Execute(w, struct {
			LastUpdateTime      string
			LastUpdateDiff      string
			SecondsToNextUpdate int64
			LastBlockHeight     int64
			CurrentBlockHeight  int64
			BlockSpeed          float64
			Upgrades            []*urproto.Upgrade
			BlocksToUpgrade     map[int64]string
			BlocksToETA         map[int64]string
			UpgradeProgress     map[int64]string
			Hostname            string
			Providers           map[int32]string
			UpgradeTypes        map[int32]string
			DefaultNetwork      string
			LogoBase64          string
			Warning             string
		}{
			LastUpdateTime:      syncInfo.LastUpdateTime.UTC().Format(time.RFC3339),
			LastUpdateDiff:      time.Since(syncInfo.LastUpdateTime).Truncate(time.Second).String(),
			SecondsToNextUpdate: int64(time.Until(syncInfo.LastUpdateTime.Add(upInterval)).Seconds()),
			LastBlockHeight:     syncInfo.LastBlockHeight,
			CurrentBlockHeight:  latestHeight,
			BlockSpeed:          blockSpeed.Seconds(),
			Upgrades:            upgrades,
			BlocksToUpgrade:     blocksToUpgradeMap,
			BlocksToETA:         blocksToETAMap,
			Hostname:            util.GetHostname(),
			Providers: map[int32]string{
				urproto.ProviderType_value["LOCAL"]:    "LOCAL",
				urproto.ProviderType_value["DATABASE"]: "DATABASE",
			},
			UpgradeTypes:   urproto.UpgradeType_name,
			DefaultNetwork: d.ur.Network(),
			LogoBase64:     base64.StdEncoding.EncodeToString(logoData),
			Warning:        warning,
		})
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			_, _ = w.Write([]byte(err.Error()))
			return
		}
	})
}

func formatRelativeTime(t time.Time) string {
	now := time.Now()
	diff := t.Sub(now)

	if diff < 0 {
		return ""
	}

	days := int(diff.Hours() / 24)
	hours := int(math.Mod(diff.Hours(), 24))
	minutes := int(math.Mod(diff.Minutes(), 60))

	if days == 0 && hours == 0 && minutes <= 1 {
		return "now"
	}
	parts := make([]string, 0, 3)

	if days > 0 {
		parts = append(parts, fmt.Sprintf("%dd", days))
	}

	if hours > 0 {
		parts = append(parts, fmt.Sprintf("%dh", hours))
	}

	if days == 0 && minutes > 0 {
		parts = append(parts, fmt.Sprintf("%dm", minutes))
	}

	return strings.Join(parts, " ")
}
