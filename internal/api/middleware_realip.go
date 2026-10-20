package api

import (
	"net"
	"net/http"
	"strings"
)

// TrustedProxyMiddleware creates a middleware that rewrites r.RemoteAddr
// based on forwarded headers (CF-Connecting-IP, X-Real-IP, X-Forwarded-For)
// only if the direct connection originates from one of the trusted proxy IP ranges.
func TrustedProxyMiddleware(trustedCIDRs []string) func(http.Handler) http.Handler {
	var parsedNets []*net.IPNet

	for _, cidr := range trustedCIDRs {
		cidr = strings.TrimSpace(cidr)
		if cidr == "" {
			continue
		}
		// If it's a single IP without mask, append /32 or /128
		if !strings.Contains(cidr, "/") {
			ip := net.ParseIP(cidr)
			if ip != nil {
				if ip.To4() != nil {
					cidr += "/32"
				} else {
					cidr += "/128"
				}
			}
		}
		_, ipNet, err := net.ParseCIDR(cidr)
		if err == nil {
			parsedNets = append(parsedNets, ipNet)
		}
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			remoteIP := extractIP(r.RemoteAddr)
			if remoteIP != nil && isTrusted(remoteIP, parsedNets) {
				// We trust this proxy, so check the headers
				if clientIP := getForwardedIP(r); clientIP != "" {
					r.RemoteAddr = clientIP
				}
			}
			next.ServeHTTP(w, r)
		})
	}
}

func extractIP(remoteAddr string) net.IP {
	host, _, err := net.SplitHostPort(remoteAddr)
	if err != nil {
		host = remoteAddr
	}
	return net.ParseIP(strings.TrimSpace(host))
}

func isTrusted(ip net.IP, trustedNets []*net.IPNet) bool {
	for _, ipNet := range trustedNets {
		if ipNet.Contains(ip) {
			return true
		}
	}
	return false
}

func getForwardedIP(r *http.Request) string {
	// 1. Cloudflare
	if cf := strings.TrimSpace(r.Header.Get("CF-Connecting-IP")); cf != "" {
		if net.ParseIP(cf) != nil {
			return cf
		}
	}

	// 2. X-Real-IP
	if xrip := strings.TrimSpace(r.Header.Get("X-Real-IP")); xrip != "" {
		if net.ParseIP(xrip) != nil {
			return xrip
		}
	}

	// 3. X-Forwarded-For (can be a comma-separated list of IPs; we take the first valid IP from the left)
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		parts := strings.Split(xff, ",")
		for _, part := range parts {
			ipStr := strings.TrimSpace(part)
			if ip := net.ParseIP(ipStr); ip != nil {
				return ipStr
			}
		}
	}

	return ""
}
