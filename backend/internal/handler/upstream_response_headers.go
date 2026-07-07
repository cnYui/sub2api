package handler

import (
	"net/http"
	"strings"
)

var upstreamHopByHopHeaders = map[string]struct{}{
	"content-length":    {},
	"transfer-encoding": {},
	"connection":        {},
}

func copyUpstreamPassthroughHeaders(dst http.Header, src http.Header) {
	for key, values := range src {
		if _, blocked := upstreamHopByHopHeaders[strings.ToLower(key)]; blocked {
			continue
		}
		for _, value := range values {
			dst.Add(key, value)
		}
	}
}
