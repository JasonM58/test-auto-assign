package autoassign

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"time"
)

// =====================
// STRUCT
// =====================

type PrometheusMetrics struct {
	BaseURL string
	Client  *http.Client
}

// =====================
// INTERNAL RESPONSE STRUCT
// =====================

type promResponse struct {
	Data struct {
		Result []struct {
			Metric map[string]string `json:"metric"`
			Value  []interface{}     `json:"value"`
		} `json:"result"`
	} `json:"data"`
}

// =====================
// PUBLIC METHODS (IMPLEMENT INTERFACE)
// =====================

// GetReviewWorkload mengambil workload reviewer berdasarkan metric
func (p *PrometheusMetrics) GetReviewWorkload(ctx context.Context) (map[string]int, error) {
	query := `sum by (reviewer)(increase(pr_reviewers_load[30m]))`
	return p.query(ctx, query)
}

// GetRecentReviewCount mengambil jumlah review terbaru
func (p *PrometheusMetrics) GetRecentReviewCount(ctx context.Context) (map[string]int, error) {
	query := `sum by (reviewer)(increase(pr_reviews_total[7d]))`
	return p.query(ctx, query)
}

// =====================
// PRIVATE HELPER
// =====================

func (p *PrometheusMetrics) query(ctx context.Context, promQL string) (map[string]int, error) {

	// =====================
	// TIMEOUT
	// =====================
	ctx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()

	// =====================
	// BUILD URL
	// =====================
	endpoint := fmt.Sprintf(
		"%s/api/v1/query?query=%s",
		p.BaseURL,
		url.QueryEscape(promQL),
	)

	// =====================
	// CREATE REQUEST
	// =====================
	req, err := http.NewRequestWithContext(ctx, "GET", endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("create request failed: %w", err)
	}

	// =====================
	// EXECUTE REQUEST
	// =====================
	resp, err := p.Client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	// =====================
	// CHECK STATUS
	// =====================
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("prometheus returned status %d", resp.StatusCode)
	}

	// =====================
	// PARSE RESPONSE
	// =====================
	var result promResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decode failed: %w", err)
	}

	// =====================
	// TRANSFORM TO DOMAIN
	// =====================
	out := make(map[string]int)

	for _, r := range result.Data.Result {

		// ambil label reviewer
		reviewer, ok := r.Metric["reviewer"]
		if !ok {
			continue
		}

		// ambil value (string)
		valStr, ok := r.Value[1].(string)
		if !ok {
			continue
		}

		// convert ke float → int
		valFloat, err := strconv.ParseFloat(valStr, 64)
		if err != nil {
			continue
		}

		out[reviewer] = int(valFloat + 0.5)
	}

	// =====================
	// DEBUG LOG (optional)
	// =====================
	fmt.Printf("[PROMETHEUS] query=%s result=%v\n", promQL, out)

	return out, nil
}
