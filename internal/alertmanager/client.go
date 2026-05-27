package alertmanager

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/yeniklas/larm02/internal/config"
)

var httpClient = &http.Client{Timeout: 10 * time.Second}

// FetchAll retrieves alert groups from all configured Alertmanager instances
// concurrently and merges the results. Groups with identical label sets are
// merged across instances; alerts within a merged group are deduplicated by
// instance+fingerprint. Errors from individual instances are collected and
// returned alongside any groups that did succeed.
func FetchAll(ctx context.Context, cfg *config.Config) ([]AlertGroup, map[string]error) {
	type result struct {
		groups []AlertGroup
		err    error
	}

	results := make([]result, len(cfg.Alertmanagers))
	var wg sync.WaitGroup

	for i, am := range cfg.Alertmanagers {
		wg.Add(1)
		go func(idx int, am config.AlertmanagerConfig) {
			defer wg.Done()
			groups, err := fetchOneGroups(ctx, am)
			results[idx] = result{groups: groups, err: err}
		}(i, am)
	}
	wg.Wait()

	type mergedGroup struct {
		group AlertGroup
		seen  map[string]struct{}
	}

	byKey := make(map[string]*mergedGroup)
	var order []string
	instErrs := make(map[string]error)

	for i, r := range results {
		if r.err != nil {
			instErrs[cfg.Alertmanagers[i].Name] = r.err
			continue
		}
		for _, g := range r.groups {
			key := groupKey(g.Labels)
			if mg, exists := byKey[key]; exists {
				for _, a := range g.Alerts {
					alertKey := a.Instance + "/" + a.Fingerprint
					if _, dup := mg.seen[alertKey]; !dup {
						mg.seen[alertKey] = struct{}{}
						mg.group.Alerts = append(mg.group.Alerts, a)
					}
				}
			} else {
				seen := make(map[string]struct{})
				alerts := make([]Alert, 0, len(g.Alerts))
				for _, a := range g.Alerts {
					alertKey := a.Instance + "/" + a.Fingerprint
					if _, dup := seen[alertKey]; !dup {
						seen[alertKey] = struct{}{}
						alerts = append(alerts, a)
					}
				}
				byKey[key] = &mergedGroup{
					group: AlertGroup{Labels: g.Labels, Receiver: g.Receiver, Alerts: alerts},
					seen:  seen,
				}
				order = append(order, key)
			}
		}
	}

	merged := make([]AlertGroup, 0, len(order))
	for _, key := range order {
		merged = append(merged, byKey[key].group)
	}
	return merged, instErrs
}

// groupKey returns a canonical string key for a set of group labels.
func groupKey(labels map[string]string) string {
	keys := make([]string, 0, len(labels))
	for k := range labels {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	parts := make([]string, 0, len(keys))
	for _, k := range keys {
		parts = append(parts, k+"="+labels[k])
	}
	return strings.Join(parts, "\x00")
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

func fetchOneGroups(ctx context.Context, am config.AlertmanagerConfig) ([]AlertGroup, error) {
	url := strings.TrimRight(am.URL, "/") + "/api/v2/alerts/groups?active=true&silenced=true&inhibited=true"
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

	var groups []AlertGroup
	if err := json.NewDecoder(resp.Body).Decode(&groups); err != nil {
		return nil, fmt.Errorf("%s: decoding response: %w", am.Name, err)
	}

	for i := range groups {
		for j := range groups[i].Alerts {
			groups[i].Alerts[j].Instance = am.Name
		}
	}
	return groups, nil
}
