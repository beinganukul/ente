//go:build js && wasm

package playstore

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"
)

type transportFunc func(*http.Request) (*http.Response, error)

func (f transportFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func TestVerifySubscription(t *testing.T) {
	const receipt = `{"expiryTimeMillis":"1740000000000","linkedPurchaseToken":"previous","unknownField":true}`
	client := &Client{http: &http.Client{Transport: transportFunc(func(r *http.Request) (*http.Response, error) {
		if r.Method != "GET" || !strings.HasSuffix(r.URL.EscapedPath(), "/tokens/token%2Fwith%2Fslashes") {
			t.Fatalf("unexpected request %s %s", r.Method, r.URL)
		}
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(receipt))}, nil
	})}}
	purchase, err := client.VerifySubscription(context.Background(), "io.ente.photos", "plan", "token/with/slashes")
	if err != nil {
		t.Fatal(err)
	}
	if purchase.ExpiryTimeMillis != 1740000000000 || purchase.LinkedPurchaseToken != "previous" {
		t.Fatalf("incorrect purchase: %+v", purchase)
	}
	data, err := json.Marshal(*purchase)
	if err != nil || string(data) != receipt {
		t.Fatalf("receipt logging lost fields: %s, %v", data, err)
	}
	client.http.Transport = transportFunc(func(r *http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: 401, Body: io.NopCloser(strings.NewReader(receipt))}, nil
	})
	if _, err := client.VerifySubscription(context.Background(), "pkg", "plan", "token"); err == nil {
		t.Fatal("unverified receipt accepted")
	}
	var missing *Client
	if _, err := missing.VerifySubscription(context.Background(), "pkg", "plan", "token"); err == nil {
		t.Fatal("missing credentials accepted")
	}
	if err := json.Unmarshal([]byte(`{"expiryTimeMillis":"9223372036854775808"}`), &SubscriptionPurchase{}); err == nil {
		t.Fatal("overflowing expiry accepted")
	}
}
