package wsproxy

import (
	"bufio"
	"bytes"
	"encoding/base64"
	"fmt"
	"net"
	"net/http"
	"strings"
)

func porxyAuth(req *http.Request) (username, password string, ok bool) {
	auth := req.Header.Get("Proxy-Authorization")
	if auth == "" {
		return
	}
	return parseBasicAuth(auth)
}

func parseBasicAuth(auth string) (username, password string, ok bool) {
	const prefix = "Basic "
	// Case insensitive prefix match. See Issue 22736.
	if len(auth) < len(prefix) || !strings.EqualFold(auth[:len(prefix)], prefix) {
		return
	}
	c, err := base64.StdEncoding.DecodeString(auth[len(prefix):])
	if err != nil {
		return
	}
	cs := string(c)
	s := strings.IndexByte(cs, ':')
	if s < 0 {
		return
	}
	return cs[:s], cs[s+1:], true
}

func makeResponse(resp *http.Response) []byte {
	buf := bytes.NewBuffer(make([]byte, 0))
	w := bufio.NewWriter(buf)

	resp.Write(w)
	w.Flush()

	return buf.Bytes()
}

// StartHttpProxy ...
func StartHttpProxy(tcpConn *net.TCPConn, handler AuthHandlerFunc,
	reader *bufio.Reader, writer *bufio.Writer) {
	// 读取client的request.
	req, err := http.ReadRequest(reader)
	if err != nil {
		fmt.Println("HttpProxy read request error", err.Error())
		return
	}

	resp := http.Response{
		ProtoMajor: req.ProtoMajor,
		ProtoMinor: req.ProtoMinor,
		Close:      false,
	}

	// 如果访问的不是CONNECT, 统一返回HTTP 200 OK.
	if req.Method != "CONNECT" {
		resp.Status = "200 OK"
		resp.StatusCode = 200
		resp.ContentLength = 0

		resp.Header = http.Header{
			"Server": []string{"nginx/1.19.0"},
		}

		writer.Write(makeResponse(&resp))
		writer.Flush()

		return
	}

	user, passwd, ok := porxyAuth(req)
	if handler != nil {
		if !ok {
			resp.Status = "407 Proxy Authentication Required"
			resp.StatusCode = 407
			resp.ContentLength = -1

			resp.Header = http.Header{
				"Proxy-Authorization": []string{"Basic realm=\"proxy\""},
			}

			writer.Write(makeResponse(&resp))
			writer.Flush()

			return
		} else if !handler(user, passwd) {
			resp.Status = "401 Unauthorized"
			resp.StatusCode = 401
			resp.ContentLength = 0

			resp.Header = http.Header{
				"Server":              []string{"nginx/1.19.0"},
				"Proxy-Authorization": []string{"Basic realm=\"proxy\""},
			}

			writer.Write(makeResponse(&resp))
			writer.Flush()

			return
		} else {
			writer.Write([]byte("HTTP/1.1 200 Connection established\r\n\r\n"))
			writer.Flush()
		}
	} else if handler == nil {
		writer.Write([]byte("HTTP/1.1 200 Connection established\r\n\r\n"))
		writer.Flush()
	}

	hostname := req.RequestURI
	fmt.Println("Start connect to:", hostname)
	targetConn, err := net.Dial("tcp", hostname)
	if err != nil {
		return
	}
	defer targetConn.Close()

	// Start proxying
	errCh := make(chan error, 2)
	go proxy(targetConn, tcpConn, errCh)
	go proxy(tcpConn, targetConn, errCh)

	// Wait
	for i := 0; i < 2; i++ {
		e := <-errCh
		if e != nil {
			break
		}
	}

	fmt.Println("Leave http proxy with client:", tcpConn.RemoteAddr())
}
