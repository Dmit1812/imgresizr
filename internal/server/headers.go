package internalhttp

import (
	"net/http"
	"strings"
)

var headersToSkip = map[string]bool{
	"content-length":    true,
	"connection":        true,
	"content-type":      true,
	"transfer-encoding": true,
	"upgrade":           true,
	"keep-alive":        true,
	"te":                true,
	"accept-ranges":     true,
}

func shouldSkipHeader(key string) bool {
	if _, ok := headersToSkip[strings.ToLower(key)]; ok {
		return true
	}
	return false
}

func CopyHeaders(h *http.Header, req *http.Request) {
	for key, values := range *h {
		for _, value := range values {
			if !shouldSkipHeader(key) {
				req.Header.Add(key, value)
			}
		}
	}
}

func cleanHeaders(h *http.Header) http.Header {
	cleaned := make(http.Header)
	if h == nil {
		return cleaned
	}
	for key, values := range *h {
		for _, value := range values {
			if !shouldSkipHeader(key) {
				cleaned.Add(key, value)
			}
		}
	}
	return cleaned
}

func writeHeaders(imageResponseHeaders *http.Header, w http.ResponseWriter) {
	for key, values := range *imageResponseHeaders {
		for _, value := range values {
			if !shouldSkipHeader(key) {
				w.Header().Add(key, value)
			}
		}
	}
}
