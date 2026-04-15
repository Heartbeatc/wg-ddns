package cloudflare

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"wg-ddns/internal/model"
)

const baseURL = "https://api.cloudflare.com/client/v4"

type Client struct {
	httpClient *http.Client
	token      string
	zoneName   string
	zoneID     string
}

type DNSRecord struct {
	ID      string `json:"id"`
	Type    string `json:"type"`
	Name    string `json:"name"`
	Content string `json:"content"`
	TTL     int    `json:"ttl"`
	Proxied bool   `json:"proxied"`
}

type RecordChange struct {
	Name   string
	Action string
	Before string
	After  string
}

type envelope[T any] struct {
	Success bool            `json:"success"`
	Errors  []responseError `json:"errors"`
	Result  T               `json:"result"`
}

type responseError struct {
	Message string `json:"message"`
}

func New(cfg model.Cloudflare) (*Client, error) {
	token := os.Getenv(cfg.TokenEnv)
	if token == "" {
		return nil, fmt.Errorf("cloudflare token env %q is empty", cfg.TokenEnv)
	}
	return &Client{
		httpClient: &http.Client{Timeout: 15 * time.Second},
		token:      token,
		zoneName:   cfg.Zone,
	}, nil
}

func (c *Client) EnsureDNSRecords(ctx context.Context, cfg model.Cloudflare, names []string, ip string, dryRun bool) ([]RecordChange, error) {
	zoneID, err := c.ensureZoneID(ctx)
	if err != nil {
		return nil, err
	}

	var changes []RecordChange
	for _, name := range names {
		record, err := c.getDNSRecord(ctx, zoneID, cfg.RecordType, name)
		if err != nil {
			return nil, err
		}

		if record == nil {
			change := RecordChange{Name: name, Action: "create", After: ip}
			changes = append(changes, change)
			if dryRun {
				continue
			}
			if err := c.createDNSRecord(ctx, zoneID, cfg, name, ip); err != nil {
				return nil, err
			}
			continue
		}

		if record.Content == ip && record.TTL == cfg.TTL && record.Proxied == cfg.Proxied {
			continue
		}

		change := RecordChange{
			Name:   name,
			Action: "update",
			Before: fmt.Sprintf("%s ttl=%d proxied=%t", record.Content, record.TTL, record.Proxied),
			After:  fmt.Sprintf("%s ttl=%d proxied=%t", ip, cfg.TTL, cfg.Proxied),
		}
		changes = append(changes, change)
		if dryRun {
			continue
		}
		if err := c.updateDNSRecord(ctx, zoneID, record.ID, cfg, name, ip); err != nil {
			return nil, err
		}
	}

	return changes, nil
}

func (c *Client) ensureZoneID(ctx context.Context) (string, error) {
	if c.zoneID != "" {
		return c.zoneID, nil
	}

	query := url.Values{}
	query.Set("name", c.zoneName)
	var resp envelope[[]struct {
		ID   string `json:"id"`
		Name string `json:"name"`
	}]
	if err := c.do(ctx, http.MethodGet, "/zones?"+query.Encode(), nil, &resp); err != nil {
		return "", err
	}
	for _, zone := range resp.Result {
		if strings.EqualFold(zone.Name, c.zoneName) {
			c.zoneID = zone.ID
			return c.zoneID, nil
		}
	}
	return "", fmt.Errorf("cloudflare zone %q not found", c.zoneName)
}

func (c *Client) getDNSRecord(ctx context.Context, zoneID, recordType, name string) (*DNSRecord, error) {
	query := url.Values{}
	query.Set("type", recordType)
	query.Set("name", name)

	var resp envelope[[]DNSRecord]
	if err := c.do(ctx, http.MethodGet, "/zones/"+zoneID+"/dns_records?"+query.Encode(), nil, &resp); err != nil {
		return nil, err
	}
	if len(resp.Result) == 0 {
		return nil, nil
	}
	return &resp.Result[0], nil
}

func (c *Client) createDNSRecord(ctx context.Context, zoneID string, cfg model.Cloudflare, name, ip string) error {
	body := map[string]any{
		"type":    cfg.RecordType,
		"name":    name,
		"content": ip,
		"ttl":     cfg.TTL,
		"proxied": cfg.Proxied,
	}
	var resp envelope[DNSRecord]
	return c.do(ctx, http.MethodPost, "/zones/"+zoneID+"/dns_records", body, &resp)
}

func (c *Client) updateDNSRecord(ctx context.Context, zoneID, recordID string, cfg model.Cloudflare, name, ip string) error {
	body := map[string]any{
		"type":    cfg.RecordType,
		"name":    name,
		"content": ip,
		"ttl":     cfg.TTL,
		"proxied": cfg.Proxied,
	}
	var resp envelope[DNSRecord]
	return c.do(ctx, http.MethodPatch, "/zones/"+zoneID+"/dns_records/"+recordID, body, &resp)
}

func (c *Client) do(ctx context.Context, method, path string, body any, out any) error {
	var payload *bytes.Reader
	if body == nil {
		payload = bytes.NewReader(nil)
	} else {
		data, err := json.Marshal(body)
		if err != nil {
			return err
		}
		payload = bytes.NewReader(data)
	}

	req, err := http.NewRequestWithContext(ctx, method, baseURL+path, payload)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+c.token)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
		return err
	}

	switch v := out.(type) {
	case *envelope[[]struct {
		ID   string `json:"id"`
		Name string `json:"name"`
	}]:
		if !v.Success {
			return fmt.Errorf("cloudflare request failed: %s", firstError(v.Errors))
		}
	case *envelope[[]DNSRecord]:
		if !v.Success {
			return fmt.Errorf("cloudflare request failed: %s", firstError(v.Errors))
		}
	case *envelope[DNSRecord]:
		if !v.Success {
			return fmt.Errorf("cloudflare request failed: %s", firstError(v.Errors))
		}
	default:
		return fmt.Errorf("unsupported cloudflare response type")
	}

	return nil
}

func firstError(errors []responseError) string {
	if len(errors) == 0 {
		return "unknown error"
	}
	return errors[0].Message
}
