package httpdelivery

import (
	"net"
	"net/http"
	"net/netip"
	"strings"
)

// InternalAccessPolicy configures the IP whitelist rules and proxy trust behavior for internal endpoints.
type InternalAccessPolicy struct {
	AllowedCIDRs      []string
	TrustProxyHeaders bool
}

type internalAccessMiddleware struct {
	next              http.Handler
	allowedPrefixes   []netip.Prefix
	trustProxyHeaders bool
}

func newInternalAccessMiddleware(next http.Handler, policy InternalAccessPolicy) http.Handler {
	return internalAccessMiddleware{
		next:              next,
		allowedPrefixes:   parseAllowedPrefixes(policy.AllowedCIDRs),
		trustProxyHeaders: policy.TrustProxyHeaders,
	}
}

func (m internalAccessMiddleware) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	clientAddr, ok := m.clientAddr(r)
	if !ok || !m.isAllowed(clientAddr) {
		writeError(w, http.StatusForbidden, "internal endpoint is not available from this network")
		return
	}

	m.next.ServeHTTP(w, r)
}

func (m internalAccessMiddleware) clientAddr(r *http.Request) (netip.Addr, bool) {
	remoteAddr, ok := parseAddrFromHostPort(r.RemoteAddr)
	if !ok {
		return netip.Addr{}, false
	}

	if !m.trustProxyHeaders || !m.isAllowed(remoteAddr) {
		return remoteAddr, true
	}

	if forwardedAddr, ok := firstForwardedForAddr(r.Header.Get("X-Forwarded-For")); ok {
		return forwardedAddr, true
	}

	if realAddr, ok := parseAddr(r.Header.Get("X-Real-IP")); ok {
		return realAddr, true
	}

	return remoteAddr, true
}

func (m internalAccessMiddleware) isAllowed(addr netip.Addr) bool {
	for _, prefix := range m.allowedPrefixes {
		if prefix.Contains(addr) {
			return true
		}
	}

	return false
}

func parseAllowedPrefixes(values []string) []netip.Prefix {
	prefixes := make([]netip.Prefix, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}

		prefix, err := netip.ParsePrefix(value)
		if err == nil {
			prefixes = append(prefixes, prefix)
			continue
		}

		if addr, ok := parseAddr(value); ok {
			prefixes = append(prefixes, netip.PrefixFrom(addr, addr.BitLen()))
		}
	}

	return prefixes
}

func parseAddrFromHostPort(value string) (netip.Addr, bool) {
	host, _, err := net.SplitHostPort(value)
	if err != nil {
		host = strings.Trim(value, "[]")
	}

	return parseAddr(host)
}

func firstForwardedForAddr(value string) (netip.Addr, bool) {
	for _, item := range strings.Split(value, ",") {
		if addr, ok := parseAddr(item); ok {
			return addr, true
		}
	}

	return netip.Addr{}, false
}

func parseAddr(value string) (netip.Addr, bool) {
	value = strings.TrimSpace(value)
	if value == "" {
		return netip.Addr{}, false
	}

	addr, err := netip.ParseAddr(strings.Trim(value, "[]"))
	if err != nil {
		return netip.Addr{}, false
	}

	if addr.Is4In6() {
		addr = addr.Unmap()
	}

	return addr, true
}
