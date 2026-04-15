package notify

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

const enrichTimeout = 5 * time.Second

// ipInfoProvider abstracts a single IP metadata lookup source.
type ipInfoProvider struct {
	name    string
	buildFn func(ip string) (*http.Request, error)
	parseFn func(body []byte) (IPInfo, error)
}

var defaultProvider = ipInfoProvider{
	name: "ipinfo.io",
	buildFn: func(ip string) (*http.Request, error) {
		url := fmt.Sprintf("https://ipinfo.io/%s/json", ip)
		req, err := http.NewRequest("GET", url, nil)
		if err != nil {
			return nil, err
		}
		req.Header.Set("Accept", "application/json")
		return req, nil
	},
	parseFn: func(body []byte) (IPInfo, error) {
		var raw struct {
			IP       string `json:"ip"`
			City     string `json:"city"`
			Region   string `json:"region"`
			Country  string `json:"country"`
			Org      string `json:"org"`
			Timezone string `json:"timezone"`
		}
		if err := json.Unmarshal(body, &raw); err != nil {
			return IPInfo{}, fmt.Errorf("decode: %w", err)
		}
		if raw.Country == "" && raw.City == "" && raw.Org == "" {
			return IPInfo{}, fmt.Errorf("empty response")
		}

		isp := raw.Org
		as := ""
		if parts := strings.SplitN(raw.Org, " ", 2); len(parts) == 2 && strings.HasPrefix(parts[0], "AS") {
			as = parts[0]
			isp = parts[1]
		}

		return IPInfo{
			Country:     raw.Country,
			CountryCode: raw.Country,
			City:        raw.City,
			ISP:         isp,
			Org:         raw.Org,
			AS:          as,
		}, nil
	},
}

// LookupIP queries ipinfo.io (HTTPS) for structured IP metadata.
// This provides the automatic enrichment data (country/city/ISP) shown in
// notifications. The iplark.com link in notifications is a separate,
// human-readable reference for manual IP quality inspection — it is not
// a data source for this function.
// Callers should treat errors as non-fatal.
func LookupIP(ctx context.Context, ip string) (IPInfo, error) {
	return lookupIPWith(ctx, ip, defaultProvider)
}

func lookupIPWith(ctx context.Context, ip string, p ipInfoProvider) (IPInfo, error) {
	ctx, cancel := context.WithTimeout(ctx, enrichTimeout)
	defer cancel()

	req, err := p.buildFn(ip)
	if err != nil {
		return IPInfo{}, fmt.Errorf("ip lookup (%s): build request: %w", p.name, err)
	}
	req = req.WithContext(ctx)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return IPInfo{}, fmt.Errorf("ip lookup (%s): request failed: %w", p.name, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return IPInfo{}, fmt.Errorf("ip lookup (%s): HTTP %d", p.name, resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return IPInfo{}, fmt.Errorf("ip lookup (%s): read body: %w", p.name, err)
	}

	info, err := p.parseFn(body)
	if err != nil {
		return IPInfo{}, fmt.Errorf("ip lookup (%s): %w", p.name, err)
	}
	return info, nil
}
