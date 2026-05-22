package alertmanager

import (
	"bytes"
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

// PostSilence creates an acknowledgement silence on the Alertmanager instance
// identified by baseURL, matching all labels of the given alert.
func PostSilence(ctx context.Context, baseURL string, alert Alert, cfg config.AcknowledgementConfig) error {
	now := time.Now().UTC()
	silence := PostableSilence{
		Matchers:  MatchersFromLabels(alert.Labels),
		StartsAt:  now,
		EndsAt:    now.Add(cfg.GetDuration()),
		CreatedBy: cfg.GetAuthor(),
		Comment:   RenderComment(cfg.GetComment()),
	}

	body, err := json.Marshal(silence)
	if err != nil {
		return fmt.Errorf("marshal silence: %w", err)
	}

	url := strings.TrimRight(baseURL, "/") + "/api/v2/silences"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	resp, err := httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("unexpected status %s", resp.Status)
	}
	return nil
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
