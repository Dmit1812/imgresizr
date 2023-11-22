package internalhttp

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

func addHTTPSToURL(url string) string {
	if !strings.HasPrefix(url, "http://") && !strings.HasPrefix(url, "https://") {
		url = "https://" + url
	}
	return url
}

func (o *Server) LoadImageFromNetwork(imageURL string, h *http.Header) ([]byte, *http.Header, error) {
	url, err := url.Parse(addHTTPSToURL(imageURL))
	if err != nil {
		return nil, nil, fmt.Errorf("invalid image URL: (url=%s)", url.RequestURI())
	}
	return o.loadImage(url, h)
}

func (o *Server) loadImage(url *url.URL, h *http.Header) ([]byte, *http.Header, error) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(o.HTTPReadTimeout)*time.Second)
	defer cancel()
	req := o.createRequest(ctx, url, h)
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, &res.Header, fmt.Errorf("error downloading image: %w", err)
	}
	defer res.Body.Close()
	if res.StatusCode != 200 {
		return nil, &res.Header,
			fmt.Errorf("error downloading image: (status=%d) (url=%s)", res.StatusCode, req.URL.RequestURI())
	}

	buf, err := io.ReadAll(res.Body)
	if err != nil {
		return nil, &res.Header,
			fmt.Errorf("unable to create image from response body: %w (url=%s)", err, req.URL.RequestURI())
	}
	return buf, &res.Header, nil
}

func copyHeaders(h *http.Header, req *http.Request) {
	for key, values := range *h {
		for _, value := range values {
			req.Header.Add(key, value)
		}
	}
}

func (o *Server) createRequest(ctx context.Context, url *url.URL, h *http.Header) *http.Request {
	req, _ := http.NewRequestWithContext(ctx, "GET", url.RequestURI(), nil)

	copyHeaders(h, req)

	// gofreq.Header.Set("User-Agent", "imgresizr "+Version)
	req.URL = url
	return req
}
