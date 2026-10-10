//go:build !js

package playstore

import (
	upstream "github.com/awa/go-iap/playstore"
	"google.golang.org/api/androidpublisher/v3"
)

type Client = upstream.Client
type DeveloperNotification = upstream.DeveloperNotification
type SubscriptionPurchase = androidpublisher.SubscriptionPurchase
type SubscriptionPurchasesAcknowledgeRequest = androidpublisher.SubscriptionPurchasesAcknowledgeRequest

var New = upstream.New

const (
	SubscriptionNotificationTypeExpired     = upstream.SubscriptionNotificationTypeExpired
	SubscriptionNotificationTypeAccountHold = upstream.SubscriptionNotificationTypeAccountHold
	SubscriptionNotificationTypeCanceled    = upstream.SubscriptionNotificationTypeCanceled
)
