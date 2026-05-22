package alertmanager

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/yeniklas/larm02/internal/config"
)

var httpClient = &http.Client{Timeout: 10 * time.Second}

// FetchAll retrieves alerts from all configured Alertmanager instances
// concurrently and merges the results. Errors from individual instances are
// collected and returned alongside any alerts that did succeed.
func FetchAll(ctx context.Context, cfg *config.Config) ([]Alert, []error) {
	type result struct {
		alerts []Alert
		err    error
	}

	results := make([]result, len(cfg.Alertmanagers))
	var wg sync.WaitGroup

	for i, am := range cfg.Alertmanagers {
		wg.Add(1)
		go func(idx int, am config.AlertmanagerConfig) {
			defer wg.Done()
			alerts, err := fetchOne(ctx, am)
			results[idx] = result{alerts: alerts, err: err}
		}(i, am)
	}
	wg.Wait()

	var merged []Alert
	var errs []error
	seen := make(map[string]struct{})

	for _, r := range results {
		if r.err != nil {
			errs = append(errs, r.err)
			continue
		}
		for _, a := range r.alerts {
			key := a.Instance + "/" + a.Fingerprint
			if _, dup := seen[key]; !dup {
				seen[key] = struct{}{}
				merged = append(merged, a)
			}
		}
	}
	return merged, errs
}

func fetchOne(ctx context.Context, am config.AlertmanagerConfig) ([]Alert, error) {
	url := strings.TrimRight(am.URL, "/") + "/api/v2/alerts?active=true&silenced=true&inhibited=true"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", am.Name, err)
	}
	req.Header.Set("Accept", "application/json")

	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", am.Name, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("%s: unexpected status %s", am.Name, resp.Status)
	}

	var alerts []Alert
	if err := json.NewDecoder(resp.Body).Decode(&alerts); err != nil {
		return nil, fmt.Errorf("%s: decoding response: %w", am.Name, err)
	}

	for i := range alerts {
		alerts[i].Instance = am.Name
	}
	return alerts, nil
}
