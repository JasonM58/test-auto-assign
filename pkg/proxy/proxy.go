package proxy

import (
	"fmt"
	"net/http"
	"net/http/httputil"
	"net/url"
)

type ProxyCache struct {
	URL   string `json:"url"`
	Token string `json:"token"`
}

func NewProxy(target *url.URL, token string) *httputil.ReverseProxy {
	proxy := httputil.NewSingleHostReverseProxy(target)

	// Modify the proxy request
	originalDirector := proxy.Director
	proxy.Director = func(req *http.Request) {
		originalDirector(req)
		// Add the JupyterLab token to the request
		req.URL.RawQuery = fmt.Sprintf("token=%s", token)
		// Set necessary headers
		req.Header.Set("X-Forwarded-Host", req.Host)
		req.Header.Set("X-Forwarded-Proto", "http")
		req.Host = target.Host
	}
	return proxy
}
