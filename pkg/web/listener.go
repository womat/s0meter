package web

import (
	"crypto/tls"
	"fmt"
	"net"
	"os"
	"strconv"
)

// Listener contains the network listener for stream-oriented protocols.
type Listener struct {
	// generic network listener for stream-oriented protocols
	net.Listener
	// flag for systemd socket activation
	hasSystemSocket bool
}

// NewListener checks if systemd socket activation is used and initializes the listener.
func NewListener(tlsConfig *tls.Config, host, port string) (Listener, error) {
	var listener Listener

	if os.Getenv("LISTEN_PID") == strconv.Itoa(os.Getpid()) {
		// systemd socket activation
		// basic idea taken from https://github.com/coreos/go-systemd/tree/main/examples/activation/httpserver
		// simplified through answer from https://stackoverflow.com/questions/68303671/systemd-socket-activation-sd-listen-fds-return-0-fd
		// "They start at 3 (SD_LISTEN_FDS_START)"
		f := os.NewFile(3, "from systemd")

		l, err := net.FileListener(f)
		if err != nil {
			return listener, fmt.Errorf("failed listening on systemd socket: %w", err)
		}

		listener.hasSystemSocket = true
		listener.Listener = tls.NewListener(l, tlsConfig)
		return listener, nil
	}

	// Create the listener if no systemd socket activation
	l, err := tls.Listen("tcp", net.JoinHostPort(host, port), tlsConfig)
	if err != nil {
		return listener, fmt.Errorf("failed listening on port %s: %w", port, err)
	}

	listener.hasSystemSocket = false
	listener.Listener = l
	return listener, nil
}

// Close closes the listener.
func (l *Listener) Close() error {
	if l.hasSystemSocket && l.Listener != nil {
		err := l.Listener.Close()
		return err
	}

	return nil
}
