//go:build !js

package controller

import (
	"context"
	"net"
	"time"
)

func lookupWebPushIPs(ctx context.Context, host string) ([]net.IP, error) {
	return net.DefaultResolver.LookupIP(ctx, "ip", host)
}
func dialWebPushContext(ctx context.Context, network, address string) (net.Conn, error) {
	return (&net.Dialer{Timeout: 10 * time.Second, KeepAlive: 30 * time.Second}).DialContext(ctx, network, address)
}
