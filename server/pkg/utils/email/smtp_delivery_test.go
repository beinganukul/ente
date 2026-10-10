package email

import (
	"bytes"
	"errors"
	"io"
	"net"
	"net/smtp"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// Use the real net/smtp protocol parser and DATA writer without an external
// mail service. Closing can fail after the server has accepted DATA and QUIT.
type deliverySMTPConn struct {
	replies  *strings.Reader
	written  bytes.Buffer
	closeErr error
	writeErr error
}

func (c *deliverySMTPConn) Read(p []byte) (int, error) { return c.replies.Read(p) }
func (c *deliverySMTPConn) Write(p []byte) (int, error) {
	if c.writeErr != nil && bytes.Contains(p, []byte("xxxxxxxx")) {
		return 0, c.writeErr
	}
	return c.written.Write(p)
}
func (c *deliverySMTPConn) Close() error                     { return c.closeErr }
func (c *deliverySMTPConn) LocalAddr() net.Addr              { return nil }
func (c *deliverySMTPConn) RemoteAddr() net.Addr             { return nil }
func (c *deliverySMTPConn) SetDeadline(time.Time) error      { return nil }
func (c *deliverySMTPConn) SetReadDeadline(time.Time) error  { return nil }
func (c *deliverySMTPConn) SetWriteDeadline(time.Time) error { return nil }

func newDeliverySMTPClient(t *testing.T, replies string, closeErr, writeErr error) (*smtp.Client, *deliverySMTPConn) {
	t.Helper()
	conn := &deliverySMTPConn{replies: strings.NewReader("220 smtp.example ready\r\n" + replies), closeErr: closeErr, writeErr: writeErr}
	client, err := smtp.NewClient(conn, "smtp.example")
	require.NoError(t, err)
	t.Cleanup(func() { _ = client.Close() })
	return client, conn
}

func TestSendWithClientAcceptedMessageIgnoresCleanupFailure(t *testing.T) {
	accepted := "250 smtp.example\r\n250 sender\r\n250 recipient\r\n354 send DATA\r\n250 queued\r\n"
	for _, tt := range []struct {
		name     string
		quit     string
		closeErr error
	}{
		{name: "successful quit", quit: "221 bye\r\n"},
		{name: "TLS close notify after 221", quit: "221 bye\r\n", closeErr: errors.New("tls: failed to send closeNotify alert (but connection was closed anyway): failed on promise: Error: Network connection lost.")},
		{name: "EOF instead of 221"},
		{name: "negative QUIT reply", quit: "500 QUIT failed\r\n"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			client, conn := newDeliverySMTPClient(t, accepted+tt.quit, tt.closeErr, nil)
			err := sendWithClient(client, nil, "sender@example.com", []string{"recipient@example.com"}, []byte("Subject: test\r\n\r\nqueued message\r\n"))
			require.NoError(t, err, "server accepted DATA; cleanup cannot make delivery fail")
			require.Contains(t, conn.written.String(), "\r\n.\r\nQUIT\r\n")
		})
	}
}

type deliverySMTPAuth struct{}

func (deliverySMTPAuth) Start(*smtp.ServerInfo) (string, []byte, error) {
	return "PLAIN", []byte("test auth"), nil
}
func (deliverySMTPAuth) Next([]byte, bool) ([]byte, error) { return nil, nil }

func TestSendWithClientRejectsFailuresBeforeAcceptance(t *testing.T) {
	ehlo := "250 smtp.example\r\n"
	mail := ehlo + "250 sender\r\n"
	rcpt := mail + "250 recipient\r\n"
	data := rcpt + "354 send DATA\r\n"
	for _, tt := range []struct {
		name     string
		replies  string
		auth     smtp.Auth
		msg      []byte
		writeErr error
	}{
		{name: "authentication", replies: "250-smtp.example\r\n250 AUTH PLAIN\r\n535 authentication failed\r\n", auth: deliverySMTPAuth{}},
		{name: "MAIL rejected", replies: ehlo + "550 sender rejected\r\n"},
		{name: "RCPT rejected", replies: mail + "550 recipient rejected\r\n"},
		{name: "DATA rejected", replies: rcpt + "550 DATA rejected\r\n"},
		{name: "DATA write failed", replies: data, msg: bytes.Repeat([]byte("x"), 8192), writeErr: io.ErrClosedPipe},
		{name: "negative final DATA completion", replies: data + "550 message rejected\r\n"},
		{name: "EOF before final DATA completion", replies: data},
	} {
		t.Run(tt.name, func(t *testing.T) {
			client, conn := newDeliverySMTPClient(t, tt.replies, nil, tt.writeErr)
			msg := tt.msg
			if msg == nil {
				msg = []byte("Subject: test\r\n\r\nmessage\r\n")
			}
			err := sendWithClient(client, tt.auth, "sender@example.com", []string{"recipient@example.com"}, msg)
			require.Error(t, err, "no final DATA acceptance was received")
			if tt.auth == nil {
				require.NotContains(t, conn.written.String(), "QUIT\r\n", "QUIT must not mask the original submission error")
			}
		})
	}
}
