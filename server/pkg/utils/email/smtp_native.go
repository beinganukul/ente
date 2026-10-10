//go:build !js

package email

import (
	"crypto/tls"
	"github.com/ente/stacktrace"
	"net"
	"net/smtp"
	"strings"
)

func sendMailWithEncryption(host, port string, auth smtp.Auth, from string, to []string, msg []byte, encryption string) error {
	addr := net.JoinHostPort(host, port)

	switch strings.ToLower(encryption) {
	case "tls", "ssl":
		tlsConfig := &tls.Config{
			ServerName: host,
			MinVersion: tls.VersionTLS12,
		}
		conn, err := tls.Dial("tcp", addr, tlsConfig)
		if err != nil {
			return stacktrace.Propagate(err, "failed to establish TLS connection")
		}
		defer conn.Close()

		client, err := smtp.NewClient(conn, host)
		if err != nil {
			return stacktrace.Propagate(err, "failed to create SMTP client over TLS")
		}
		defer client.Close()

		return sendWithClient(client, auth, from, to, msg)

	default:
		return smtp.SendMail(addr, auth, from, to, msg)
	}
}
