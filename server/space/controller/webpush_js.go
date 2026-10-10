//go:build js && wasm

package controller

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"

	"github.com/syumai/workers-go/cloudflare/sockets"
)

func lookupWebPushIPs(ctx context.Context, host string) ([]net.IP, error) {
	if ip := net.ParseIP(host); ip != nil {
		return []net.IP{ip}, nil
	}
	var addresses []net.IP
	for _, kind := range []string{"A", "AAAA"} {
		request, err := http.NewRequestWithContext(ctx, "GET", "https://cloudflare-dns.com/dns-query?name="+url.QueryEscape(host)+"&type="+kind, nil)
		if err != nil {
			return nil, err
		}
		request.Header.Set("Accept", "application/dns-json")
		response, err := http.DefaultClient.Do(request)
		if err != nil {
			return nil, err
		}
		var answer struct {
			Status int
			Answer []struct {
				Type int
				Data string
			}
		}
		err = json.NewDecoder(io.LimitReader(response.Body, 64*1024)).Decode(&answer)
		response.Body.Close()
		if response.StatusCode != http.StatusOK {
			return nil, fmt.Errorf("web push DNS status: %d", response.StatusCode)
		}
		if err != nil {
			return nil, err
		}
		if answer.Status != 0 {
			return nil, fmt.Errorf("web push DNS failure: %d", answer.Status)
		}
		for _, record := range answer.Answer {
			if record.Type == 1 || record.Type == 28 {
				ip := net.ParseIP(record.Data)
				if ip == nil {
					return nil, fmt.Errorf("invalid web push DNS address")
				}
				addresses = append(addresses, ip)
			}
		}
	}
	if len(addresses) == 0 {
		return nil, fmt.Errorf("web push endpoint has no addresses")
	}
	return addresses, nil
}
func dialWebPushContext(ctx context.Context, _, address string) (net.Conn, error) {
	return sockets.Connect(ctx, address, nil)
}
