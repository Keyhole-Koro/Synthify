package sourceio

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"net/url"
	"os"
	"sync/atomic"
	"testing"
)

// productionFetchClient is fetchClient as the package initialises it.
var productionFetchClient *http.Client

func TestMain(m *testing.M) {
	productionFetchClient = fetchClient
	// The other fetch tests serve images from httptest servers on 127.0.0.1,
	// which the production client refuses by design.
	fetchClient = &http.Client{}
	os.Exit(m.Run())
}

func TestFetchAddrAllowed(t *testing.T) {
	allowed := []string{
		"8.8.8.8", "1.1.1.1", "142.250.196.110", "11.0.0.1",
		"100.63.255.255", "100.128.0.1", "172.15.255.255", "172.32.0.1", "192.167.255.255", "192.169.0.1",
		"2001:4860:4860::8888", "2606:4700:4700::1111",
	}
	refused := []string{
		"127.0.0.1", "127.1.2.3", "::1", // loopback
		"10.0.0.1", "172.16.0.1", "172.31.255.255", "192.168.1.1", "fc00::1", "fd12:3456::1", // private
		"169.254.169.254", "169.254.0.1", "fe80::1", "fe80::1%eth0", // link-local, incl. the metadata server
		"100.64.0.1", "100.127.255.255", // carrier-grade NAT
		"0.0.0.0", "0.1.2.3", "::", // unspecified / this network
		"224.0.0.1", "ff02::1", // multicast
		"240.0.0.1", "255.255.255.255", "198.18.0.1", "192.0.0.1", // reserved
		"::ffff:127.0.0.1", "::ffff:10.0.0.1", "::ffff:169.254.169.254", // IPv4-mapped
		"::127.0.0.1", "64:ff9b::a00:1", "64:ff9b:1::1", "fec0::1", // IPv4-compatible, NAT64, site-local
	}
	for _, s := range allowed {
		if !fetchAddrAllowed(netip.MustParseAddr(s)) {
			t.Errorf("fetchAddrAllowed(%s) = false, want true", s)
		}
	}
	for _, s := range refused {
		if fetchAddrAllowed(netip.MustParseAddr(s)) {
			t.Errorf("fetchAddrAllowed(%s) = true, want false", s)
		}
	}
	if fetchAddrAllowed(netip.Addr{}) {
		t.Error("the zero Addr must be refused")
	}
}

func TestRefuseNonPublicAddress(t *testing.T) {
	for _, address := range []string{"127.0.0.1:80", "[::1]:443", "169.254.169.254:80", "not-an-address", "example.com:80"} {
		if err := refuseNonPublicAddress("tcp", address, nil); !errors.Is(err, errFetchAddressNotAllowed) {
			t.Errorf("refuseNonPublicAddress(%q) = %v, want errFetchAddressNotAllowed", address, err)
		}
	}
	for _, address := range []string{"8.8.8.8:443", "[2001:4860:4860::8888]:80"} {
		if err := refuseNonPublicAddress("tcp", address, nil); err != nil {
			t.Errorf("refuseNonPublicAddress(%q) = %v, want nil", address, err)
		}
	}
}

// extract_text hands a model-chosen http(s) file_uri to Load, which fetches it
// with fetchClient. The production client must not connect to loopback,
// whether the URL names the IP or a hostname that resolves to it, and the
// server must never see the request.
func TestProductionFetchClientRefusesInternalAddresses(t *testing.T) {
	var hits atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		w.Header().Set("Content-Type", "image/png")
		_, _ = w.Write([]byte("\x89PNG\r\n\x1a\n"))
	}))
	defer srv.Close()
	u, err := url.Parse(srv.URL)
	if err != nil {
		t.Fatal(err)
	}

	for _, uri := range []string{srv.URL + "/image.png", "http://localhost:" + u.Port() + "/image.png"} {
		resp, err := productionFetchClient.Get(uri)
		if resp != nil {
			_ = resp.Body.Close()
		}
		if !errors.Is(err, errFetchAddressNotAllowed) {
			t.Errorf("GET %s error = %v, want errFetchAddressNotAllowed", uri, err)
		}
	}
	if n := hits.Load(); n != 0 {
		t.Errorf("the internal server received %d requests, want 0", n)
	}
}

func TestProductionFetchClientIgnoresProxyEnvironment(t *testing.T) {
	transport, ok := productionFetchClient.Transport.(*http.Transport)
	if !ok {
		t.Fatalf("fetchClient.Transport is %T, want *http.Transport", productionFetchClient.Transport)
	}
	if transport.Proxy != nil {
		t.Error("fetchClient must dial the server itself so the address check applies to it, not to a proxy")
	}
}
