//go:build js && wasm

package email

import (
	"context"
	"crypto/tls"
	"fmt"
	"net"
	"net/smtp"
	"strings"
	"time"

	"github.com/syumai/workers-go/cloudflare/sockets"
)

func sendMailWithEncryption(host, port string, auth smtp.Auth, from string, to []string, msg []byte, encryption string) error {
	if port == "25" {
		return fmt.Errorf("Workers blocks SMTP port 25; configure port 465 or 587")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	conn, err := sockets.Connect(ctx, net.JoinHostPort(host, port), nil)
	if err != nil {
		return err
	}
	defer conn.Close()
	config := &tls.Config{ServerName: host, MinVersion: tls.VersionTLS12}
	encrypted := strings.EqualFold(encryption, "tls") || strings.EqualFold(encryption, "ssl")
	if encrypted {
		secure := tls.Client(conn, config)
		if err := secure.HandshakeContext(ctx); err != nil {
			return err
		}
		conn = secure
	}
	client, err := smtp.NewClient(conn, host)
	if err != nil {
		return err
	}
	defer client.Close()
	if !encrypted {
		if ok, _ := client.Extension("STARTTLS"); ok {
			if err := client.StartTLS(config); err != nil {
				return err
			}
		}
	}
	return sendWithClient(client, auth, from, to, msg)
}
