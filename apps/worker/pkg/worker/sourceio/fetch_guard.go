package sourceio

import (
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/netip"
	"syscall"
	"time"
)

// errFetchAddressNotAllowed reports that an external fetch tried to connect
// to an address that is not on the public internet.
var errFetchAddressNotAllowed = errors.New("external fetch: address not allowed")

// blockedFetchPrefixes lists the non-public ranges that netip.Addr's own
// predicates (unspecified, loopback, private, link-local, multicast) miss.
var blockedFetchPrefixes = []netip.Prefix{
	netip.MustParsePrefix("0.0.0.0/8"),      // "this network"; 0.0.0.0 reaches the local host
	netip.MustParsePrefix("100.64.0.0/10"),  // carrier-grade NAT (RFC 6598)
	netip.MustParsePrefix("192.0.0.0/24"),   // IETF protocol assignments (RFC 6890)
	netip.MustParsePrefix("198.18.0.0/15"),  // benchmarking (RFC 2544)
	netip.MustParsePrefix("240.0.0.0/4"),    // reserved, including 255.255.255.255
	netip.MustParsePrefix("::/96"),          // IPv4-compatible IPv6 (deprecated)
	netip.MustParsePrefix("64:ff9b::/96"),   // NAT64 (RFC 6052): reaches IPv4 through a translator
	netip.MustParsePrefix("64:ff9b:1::/48"), // local-use NAT64 (RFC 8215)
	netip.MustParsePrefix("fec0::/10"),      // site-local (deprecated)
}

// fetchAddrAllowed reports whether an external fetch may connect to addr.
// Only public unicast addresses are allowed: the URL comes from the model, so
// it must not reach the metadata server, localhost or other internal
// services (fake-gcs and the like on a self-hosted stack).
func fetchAddrAllowed(addr netip.Addr) bool {
	addr = addr.WithZone("").Unmap() // ::ffff:127.0.0.1 is 127.0.0.1
	if !addr.IsValid() || addr.IsUnspecified() || addr.IsLoopback() || addr.IsPrivate() ||
		addr.IsLinkLocalUnicast() || addr.IsMulticast() {
		return false
	}
	for _, p := range blockedFetchPrefixes {
		if p.Contains(addr) {
			return false
		}
	}
	return true
}

// refuseNonPublicAddress is the net.Dialer Control hook of fetchClient. It
// runs for every connection attempt with the resolved IP, so a redirect, or a
// public hostname that resolves (or rebinds) to an internal address, is
// refused as well.
func refuseNonPublicAddress(_, address string, _ syscall.RawConn) error {
	host, _, err := net.SplitHostPort(address)
	if err != nil {
		return fmt.Errorf("%w: %q", errFetchAddressNotAllowed, address)
	}
	addr, err := netip.ParseAddr(host)
	if err != nil || !fetchAddrAllowed(addr) {
		return fmt.Errorf("%w: %s", errFetchAddressNotAllowed, host)
	}
	return nil
}

// newGuardedFetchClient returns an HTTP client like the default one whose
// connections can only go to public addresses. It ignores HTTP(S)_PROXY so
// the address check applies to the server itself, not to a proxy.
func newGuardedFetchClient() *http.Client {
	dialer := &net.Dialer{
		Timeout:   30 * time.Second,
		KeepAlive: 30 * time.Second,
		Control:   refuseNonPublicAddress,
	}
	var transport *http.Transport
	if t, ok := http.DefaultTransport.(*http.Transport); ok {
		transport = t.Clone()
	} else {
		transport = &http.Transport{ForceAttemptHTTP2: true, TLSHandshakeTimeout: 10 * time.Second}
	}
	transport.Proxy = nil
	transport.DialContext = dialer.DialContext
	return &http.Client{Transport: transport}
}
