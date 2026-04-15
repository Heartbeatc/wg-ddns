package notify

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"
)

func TestLookupIPWithFakeProvider(t *testing.T) {
	fake := ipInfoProvider{
		name: "fake",
		buildFn: func(ip string) (*http.Request, error) {
			return http.NewRequest("GET", "https://example.com/"+ip, nil)
		},
		parseFn: func(body []byte) (IPInfo, error) {
			return IPInfo{
				Country:     "US",
				CountryCode: "US",
				City:        "San Jose",
				ISP:         "DMIT",
				Org:         "AS906 DMIT",
				AS:          "AS906",
			}, nil
		},
	}

	transport := &fakeRoundTripper{
		statusCode: 200,
		body:       `{}`,
	}
	origClient := http.DefaultClient
	http.DefaultClient = &http.Client{Transport: transport}
	defer func() { http.DefaultClient = origClient }()

	info, err := lookupIPWith(context.Background(), "1.2.3.4", fake)
	if err != nil {
		t.Fatalf("lookupIPWith() error = %v", err)
	}
	if info.Country != "US" {
		t.Errorf("Country = %q, want US", info.Country)
	}
	if info.City != "San Jose" {
		t.Errorf("City = %q, want San Jose", info.City)
	}
}

func TestLookupIPWithHTTPError(t *testing.T) {
	fake := ipInfoProvider{
		name: "fake",
		buildFn: func(ip string) (*http.Request, error) {
			return http.NewRequest("GET", "https://example.com/"+ip, nil)
		},
		parseFn: func(body []byte) (IPInfo, error) {
			return IPInfo{}, nil
		},
	}

	transport := &fakeRoundTripper{
		statusCode: 429,
		body:       `rate limited`,
	}
	origClient := http.DefaultClient
	http.DefaultClient = &http.Client{Transport: transport}
	defer func() { http.DefaultClient = origClient }()

	_, err := lookupIPWith(context.Background(), "1.2.3.4", fake)
	if err == nil {
		t.Fatal("expected error for HTTP 429")
	}
	if !strings.Contains(err.Error(), "429") {
		t.Errorf("error should contain 429, got: %v", err)
	}
}

func TestDefaultProviderParseFn(t *testing.T) {
	body := []byte(`{"ip":"1.2.3.4","city":"San Jose","region":"California","country":"US","org":"AS906 DMIT Inc","timezone":"America/Los_Angeles"}`)
	info, err := defaultProvider.parseFn(body)
	if err != nil {
		t.Fatalf("parseFn() error = %v", err)
	}
	if info.Country != "US" {
		t.Errorf("Country = %q, want US", info.Country)
	}
	if info.City != "San Jose" {
		t.Errorf("City = %q, want San Jose", info.City)
	}
	if info.AS != "AS906" {
		t.Errorf("AS = %q, want AS906", info.AS)
	}
	if info.ISP != "DMIT Inc" {
		t.Errorf("ISP = %q, want DMIT Inc", info.ISP)
	}
}

func TestDefaultProviderParseFnEmptyResponse(t *testing.T) {
	body := []byte(`{}`)
	_, err := defaultProvider.parseFn(body)
	if err == nil {
		t.Fatal("expected error for empty response")
	}
}

type fakeRoundTripper struct {
	statusCode int
	body       string
}

func (f *fakeRoundTripper) RoundTrip(*http.Request) (*http.Response, error) {
	return &http.Response{
		StatusCode: f.statusCode,
		Body:       io.NopCloser(strings.NewReader(f.body)),
		Header:     make(http.Header),
	}, nil
}
