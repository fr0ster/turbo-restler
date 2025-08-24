package rest_api_test

import (
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"sync/atomic"
	"time"
)

// StartHandlerServer starts an httptest server with a provided handler.
func StartHandlerServer(h http.HandlerFunc) *httptest.Server {
	return httptest.NewServer(h)
}

// StartStaticJSONServer starts a server that always responds with given status and JSON body.
func StartStaticJSONServer(status int, body string) *httptest.Server {
	return StartHandlerServer(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		_, _ = w.Write([]byte(body))
	})
}

// StartForwardProxy starts a minimal forward HTTP proxy.
// It returns the server, a pointer to hits counter, and the parsed proxy URL for Transport.
func StartForwardProxy() (srv *httptest.Server, hits *int32, proxyURL *url.URL) {
	var counter int32
	proxy := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&counter, 1)
		// Forward the request using absolute URL present in r.URL
		outReq, err := http.NewRequestWithContext(r.Context(), r.Method, r.URL.String(), r.Body)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadGateway)
			return
		}
		outReq.Header = r.Header.Clone()

		resp, err := http.DefaultTransport.RoundTrip(outReq)
		if err != nil {
			http.Error(w, fmt.Sprintf("proxy error: %v", err), http.StatusBadGateway)
			return
		}
		defer resp.Body.Close()
		for k, vv := range resp.Header {
			for _, v := range vv {
				w.Header().Add(k, v)
			}
		}
		w.WriteHeader(resp.StatusCode)
		_, _ = io.Copy(w, resp.Body)
	}))

	u, _ := url.Parse(proxy.URL)
	return proxy, &counter, u
}

// ProxyHits safely returns number of requests observed by the proxy.
func ProxyHits(h *int32) int32 {
	return atomic.LoadInt32(h)
}

// NewHTTPClientWithProxy creates *http.Client with provided proxy URL and timeout.
// If baseTransport is nil, a new *http.Transport is created; otherwise baseTransport is used.
func NewHTTPClientWithProxy(proxyURL *url.URL, timeout time.Duration, baseTransport http.RoundTripper) *http.Client {
	var tr http.RoundTripper
	if baseTransport != nil {
		tr = baseTransport
		if t, ok := tr.(*http.Transport); ok {
			t.Proxy = http.ProxyURL(proxyURL)
		}
	} else {
		tr = &http.Transport{Proxy: http.ProxyURL(proxyURL)}
	}
	return &http.Client{Timeout: timeout, Transport: tr}
}
