package main

import (
	"bufio"
	"log"
	"net"
	"net/http"
	"time"
)

const (
	connectionEstablished = "200 Connection Established"
)

// HTTPHandler is the http implementation of Handler.
type HTTPHandler struct {
	Dialer Dialer
}

// Handle responses http tunnel request.
func (h *HTTPHandler) Handle(conn net.Conn) {
	defer conn.Close()
	req, err := http.ReadRequest(bufio.NewReader(conn))
	if err != nil {
		log.Printf("[http] %s -> %s: valid request", conn.RemoteAddr(), conn.LocalAddr())
		return
	}

	defer req.Body.Close()
	h.handleRequest(conn, req)
}

func (h *HTTPHandler) handleRequest(conn net.Conn, req *http.Request) {
	/*
		resp := &http.Response{
			ProtoMajor: 1,
			ProtoMinor: 1,
			Header:     http.Header{},
		}

			if req.URL.Scheme != "http" {
				resp.StatusCode = http.StatusBadRequest
				resp.Write(conn)
				return
			}
	*/

	if req.Method == http.MethodConnect {
		h.handleTunnelRequest(conn, req)
	} else {
		h.handleProxyRequest(conn, req)
	}
}

func (h *HTTPHandler) handleTunnelRequest(conn net.Conn, req *http.Request) {
	host := req.Host
	if _, port, _ := net.SplitHostPort(host); port == "" {
		host = net.JoinHostPort(host, "80")
	}

	resp := &http.Response{
		ProtoMajor: 1,
		ProtoMinor: 1,
		Header:     http.Header{},
	}

	cc, err := h.Dialer.Dial("tcp", host)
	if err != nil {
		resp.StatusCode = http.StatusServiceUnavailable
		log.Printf("[http] %s -> %s -> %s: tcp connect failed", conn.RemoteAddr(), conn.LocalAddr(), host)
		resp.Write(conn)
		return
	}
	defer cc.Close()

	resp.StatusCode = http.StatusOK
	resp.Status = connectionEstablished
	resp.Header = http.Header{}

	resp.Write(conn)
	log.Printf("[http] %s -> %s: success", conn.RemoteAddr(), host)
	transport(conn, cc)
	log.Printf("[http] %s -> %s: closed", conn.RemoteAddr(), host)
}

func (h *HTTPHandler) handleProxyRequest(conn net.Conn, req *http.Request) {
	host := req.Host
	if _, port, _ := net.SplitHostPort(host); port == "" {
		host = net.JoinHostPort(host, "80")
	}

	//proxyConnection := req.Header.Get("Proxy-Connection")
	req.Header.Del("Proxy-Connection")
	req.RequestURI = ""

	tr := &http.Transport{
		MaxIdleConns:       2,
		IdleConnTimeout:    30 * time.Second,
		DisableCompression: true,
	}
	client := &http.Client{Transport: tr}
	resp, err := client.Do(req)
	if err != nil {
		log.Printf("[http] %s -> %s: server error: %v", conn.RemoteAddr(), host, err)
		resp := &http.Response{
			ProtoMajor: 1,
			ProtoMinor: 1,
			Header:     http.Header{},
		}
		resp.StatusCode = http.StatusServiceUnavailable
		resp.Write(conn)
		return
	}

	err = resp.Write(conn)
	if err != nil {
		log.Printf("[http] %s -> %s: client error: %v", conn.RemoteAddr(), host, err)
		return
	}

	go func() {
		for {
			req, err := http.ReadRequest(bufio.NewReader(conn))
			if err != nil {
				log.Printf("[http] %s -> %s: closed", conn.RemoteAddr(), host)
				return
			}

			req.Header.Del("Proxy-Connection")
			req.RequestURI = ""

			resp, err := client.Do(req)
			if err != nil {
				resp.StatusCode = http.StatusServiceUnavailable
				log.Printf("[http] %s -> %s: closed", conn.RemoteAddr(), host)
				return
			}

			err = resp.Write(conn)
			if err != nil {
				log.Printf("[http] %s -> %s: closed", conn.RemoteAddr(), host)
				return
			}
		}
	}()
}
