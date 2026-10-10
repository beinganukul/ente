//go:build js && wasm

package playstore

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"golang.org/x/oauth2/google"
)

type Client struct{ http *http.Client }
type SubscriptionPurchase struct {
	ExpiryTimeMillis    int64  `json:"expiryTimeMillis,string"`
	LinkedPurchaseToken string `json:"linkedPurchaseToken"`
	raw                 json.RawMessage
}

func (p *SubscriptionPurchase) UnmarshalJSON(data []byte) error {
	type purchase SubscriptionPurchase
	if err := json.Unmarshal(data, (*purchase)(p)); err != nil {
		return err
	}
	p.raw = append(p.raw[:0], data...)
	return nil
}
func (p SubscriptionPurchase) MarshalJSON() ([]byte, error) { return p.raw, nil }

type SubscriptionPurchasesAcknowledgeRequest struct{}

type DeveloperNotification struct {
	Version                  string `json:"version"`
	PackageName              string `json:"packageName"`
	EventTimeMillis          string `json:"eventTimeMillis"`
	SubscriptionNotification struct {
		Version          string `json:"version"`
		NotificationType int    `json:"notificationType,omitempty"`
		PurchaseToken    string `json:"purchaseToken,omitempty"`
		SubscriptionID   string `json:"subscriptionId,omitempty"`
	} `json:"subscriptionNotification,omitempty"`
	OneTimeProductNotification json.RawMessage `json:"oneTimeProductNotification,omitempty"`
	TestNotification           struct {
		Version string `json:"version"`
	} `json:"testNotification,omitempty"`
}

const (
	SubscriptionNotificationTypeCanceled    = 3
	SubscriptionNotificationTypeAccountHold = 5
	SubscriptionNotificationTypeExpired     = 13
)

func New(credentials []byte) (*Client, error) {
	config, err := google.JWTConfigFromJSON(credentials, "https://www.googleapis.com/auth/androidpublisher")
	if err != nil {
		return nil, err
	}
	client := config.Client(context.Background())
	client.Timeout = 30 * time.Second
	return &Client{http: client}, nil
}

func (c *Client) request(ctx context.Context, method, pkg, subscription, token, action string) ([]byte, error) {
	if c == nil {
		return nil, fmt.Errorf("Play Store credentials are not configured")
	}
	endpoint := "https://androidpublisher.googleapis.com/androidpublisher/v3/applications/" + url.PathEscape(pkg) + "/purchases/subscriptions/" + url.PathEscape(subscription) + "/tokens/" + url.PathEscape(token) + action
	var body io.Reader
	if method == http.MethodPost {
		body = strings.NewReader("{}")
	}
	req, err := http.NewRequestWithContext(ctx, method, endpoint, body)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	response, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return nil, fmt.Errorf("Play Store API returned HTTP %d", response.StatusCode)
	}
	return io.ReadAll(io.LimitReader(response.Body, 1<<20))
}
func (c *Client) VerifySubscription(ctx context.Context, pkg, subscription, token string) (*SubscriptionPurchase, error) {
	data, err := c.request(ctx, http.MethodGet, pkg, subscription, token, "")
	if err != nil {
		return nil, err
	}
	var purchase SubscriptionPurchase
	if err := json.Unmarshal(data, &purchase); err != nil {
		return nil, err
	}
	return &purchase, nil
}
func (c *Client) AcknowledgeSubscription(ctx context.Context, pkg, subscription, token string, _ *SubscriptionPurchasesAcknowledgeRequest) error {
	_, err := c.request(ctx, http.MethodPost, pkg, subscription, token, ":acknowledge")
	return err
}
func (c *Client) CancelSubscription(ctx context.Context, pkg, subscription, token string) error {
	_, err := c.request(ctx, http.MethodPost, pkg, subscription, token, ":cancel")
	return err
}
