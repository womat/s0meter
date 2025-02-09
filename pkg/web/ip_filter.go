package web

import (
	"errors"
	"log/slog"
	"net"
	"net/http"
	"strings"
)

var (
	ErrForbidden = errors.New("forbidden")
)

// WithIPFilter is a middleware function that filters IP addresses for incoming HTTP requests.
// It returns an http.Handler that checks if the IP address of the incoming request is allowed to access the application.
// If the IP address is not allowed, it returns a 403 Forbidden response.
// If the IP address is allowed, it calls the next handler, which is responsible for writing the response.
//
// The middleware can be configured with two parameters: `allowedIPs` and `blockedIPs`.
// Note: `blockedIPs` has priority over `allowedIPs`.
// If an IP address is in the `blockedIPs` list, it is forbidden to access the application, regardless of whether it is in the `allowedIPs` list.
// To allow access from all IP addresses, set `allowedIPs` to ["ALL"].
//
// `allowedIPs` is a slice of strings containing IP addresses or networks that are allowed to access the application.
// The value "ALL" allows access from all IP addresses/networks (same as an empty list).
// If the allowlist is empty, all IP addresses are allowed.
// Multiple IP addresses or networks can be defined, for example,
//   - 127.0.0.1
//   - ::1
//   - 192.168.0.0/16
//   - 10.0.0.0/8
//
// `blockedIPs` is a slice of strings containing IP addresses or networks that are forbidden to access the application.
// The default is empty, meaning no IP addresses are blocked.
// Multiple IP addresses or networks can be defined, for example,
//   - 192.168.0.1
//   - 192.168.0.0/16
//   - 10.0.0.0/8
//   - 192.168.254.15
//
// Note: `::1` is the IPv6 loopback address.
func WithIPFilter(h http.Handler, allowedIPs, blockedIPs []string) http.Handler {
	// If both allowedIPs and blockedIPs are empty, no IP filtering is necessary, return the handler as is
	if len(allowedIPs) == 0 && len(blockedIPs) == 0 {
		return h
	}

	return http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {

			remoteAddr, _, err := net.SplitHostPort(r.RemoteAddr)
			if err != nil {
				slog.Error("Invalid remote address", "remoteAddress", r.RemoteAddr, "error", err)
				Encode(w, http.StatusInternalServerError, NewApiError(errors.New("invalid remote address")))
				return
			}

			slog.Debug("Checking IP address against IP Filter", "remoteAddress", remoteAddr, "method", r.Method, "path", r.URL.Path)

			if isIPBlocked(remoteAddr, blockedIPs) {
				slog.Warn("IP Address is a blocked IP/Network, return status 403", "remoteAddress", remoteAddr)
				Encode(w, http.StatusForbidden, NewApiError(ErrForbidden))
				return
			}

			// If the IP is not in the blocklist, check the allowlist
			if !isIPAllowed(remoteAddr, allowedIPs) {
				slog.Warn("IP Address not an allowed IP/Network, return status 403", "remoteAddress", remoteAddr)
				Encode(w, http.StatusForbidden, NewApiError(ErrForbidden))
				return
			}

			// If the IP is allowed, proceed with the next handler
			h.ServeHTTP(w, r)
		},
	)
}

// isIPBlocked checks if IP is in blocklist
func isIPBlocked(ip string, blockedIPs []string) bool {

	// If the blockedIPs is empty, nothing is blocked
	if len(blockedIPs) == 0 {
		return false
	}

	parsedIP := net.ParseIP(ip)
	if parsedIP == nil {
		return false
	}

	for _, blockedIP := range blockedIPs {
		if isMatchingIP(parsedIP, blockedIP) {
			return true
		}
	}

	return false
}

// isIPAllowed checks if IP is in allowlist
func isIPAllowed(ip string, allowedIPs []string) bool {

	// If the allowlist is empty, allow all IPs
	if len(allowedIPs) == 0 {
		return true
	}

	parsedIP := net.ParseIP(ip)
	if parsedIP == nil {
		return false
	}

	// Check if the IP is in the allowlist
	for _, allowedIP := range allowedIPs {
		if allowedIP == "ALL" || isMatchingIP(parsedIP, allowedIP) {
			return true
		}
	}

	return false
}

// Function to check if the IP or network matches the template
func isMatchingIP(parsedIP net.IP, template string) bool {
	if strings.Contains(template, "/") {
		if _, ipNet, err := net.ParseCIDR(template); err == nil {
			return ipNet.Contains(parsedIP)
		}
		return false
	}

	// Ensure the template IP is valid
	templateIP := net.ParseIP(template)
	return templateIP != nil && parsedIP.Equal(templateIP)
}
