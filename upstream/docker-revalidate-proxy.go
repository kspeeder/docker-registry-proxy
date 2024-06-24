package main

import (
	"fmt"
	"net/http"
	"net/http/httputil"
	"net/url"
)

type RevalidateWithHEAD struct {
	DefaultTransport http.RoundTripper
}

func (self *RevalidateWithHEAD) RoundTrip(req *http.Request) (*http.Response, error) {
	if req.Method == "GET" {
		etag := req.Header.Get("If-None-Match")

		if etag != "" {
			req.Method = "HEAD"
			resp, err := self.DefaultTransport.RoundTrip(req)
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
	return self.DefaultTransport.RoundTrip(req)
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
	fmt.Println("start docker registry proxy server on", localAddr)
	proxyServer.ListenAndServe()
}
