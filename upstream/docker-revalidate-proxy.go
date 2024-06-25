package main

import (
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"
)

var logger = log.Default()

type RevalidateWithHEAD struct {
	DefaultTransport http.RoundTripper
	Remaining        string
}

func (self *RevalidateWithHEAD) LogRoundTrip(req *http.Request) (*http.Response, error) {
	resp, err := self.DefaultTransport.RoundTrip(req)
	if err == nil {
		remaining := resp.Header.Get("ratelimit-remaining")
		if remaining != "" && remaining != self.Remaining {
			logger.Printf("ratelimit-remaining: %s\n", remaining)
			self.Remaining = remaining
		}
	}
	return resp, err
}
func (self *RevalidateWithHEAD) RoundTrip(req *http.Request) (*http.Response, error) {
	if req.Method == "GET" {
		etag := req.Header.Get("If-None-Match")

		if etag != "" {
			req.Method = "HEAD"
			resp, err := self.LogRoundTrip(req)
			req.Method = "GET"

			if err != nil || resp.StatusCode != 200 {
				if err == nil && resp.StatusCode != 304 {
					resp.Header.Set("Content-Length", "0")
					resp.ContentLength = 0
				}
				return resp, err
			}

			newetag := resp.Header.Get("ETag")

			if newetag == etag {
				resp.StatusCode = 304
				resp.Status = "304 Not Modified"
				return resp, nil
			}
			if resp.Body != nil {
				resp.Body.Close()
			}
		}
	}
	return self.LogRoundTrip(req)
}

func newDockerReverseProxy(target *url.URL) *httputil.ReverseProxy {
	r := httputil.NewSingleHostReverseProxy(target)
	//r.BufferPool = &proxyPool{}
	old := r.Director
	r.Director = func(req *http.Request) {
		old(req)
		req.RemoteAddr = ""
		// The host is changed here
		req.Host = target.Host
	}
	r.Transport = &RevalidateWithHEAD{
		DefaultTransport: http.DefaultTransport,
	}

	return r
}

func main() {
	localAddr := "127.0.0.1:5000"

	dockerURL, _ := url.Parse("https://registry-1.docker.io")
	dockerReverse := newDockerReverseProxy(dockerURL)
	proxyServer := &http.Server{
		Addr:    localAddr,
		Handler: dockerReverse,
	}
	defer proxyServer.Close()
	logger.Printf("start docker registry proxy server on %s\n", localAddr)
	proxyServer.ListenAndServe()
}
