//go:build js && wasm

package controller

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"
)

type dnsTransport func(*http.Request) (*http.Response, error)

func (f dnsTransport) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func TestWorkerWebPushDNS(t *testing.T) {
	original := http.DefaultClient
	defer func() { http.DefaultClient = original }()
	for _, test := range []struct {
		body     string
		rejected bool
	}{
		{`{"Status":0,"Answer":[{"type":1,"data":"8.8.8.8"}]}`, false},
		{`{"Status":3}`, true},
		{`{"Status":0,"Answer":[{"type":1,"data":"invalid"}]}`, true},
	} {
		http.DefaultClient = &http.Client{Transport: dnsTransport(func(r *http.Request) (*http.Response, error) {
			if r.URL.Host != "cloudflare-dns.com" || r.URL.Query().Get("name") != "push.example.com" || r.Header.Get("Accept") != "application/dns-json" {
				t.Fatalf("unexpected DNS request: %s", r.URL)
			}
			return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(test.body))}, nil
		})}
		addresses, err := lookupWebPushIPs(context.Background(), "push.example.com")
		if test.rejected {
			if err == nil {
				t.Fatal("accepted invalid DNS result")
			}
		} else if err != nil || len(addresses) == 0 {
			t.Fatalf("lookup: %v, %v", addresses, err)
		}
	}
}
