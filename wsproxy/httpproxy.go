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

var (
	hs407 = "HTTP/1.1 407 Proxy Authentication Required\r\nProxy-Authorization: Basic realm=\"proxy\"\r\n\r\n"
	hs401 = "HTTP/1.1 401 Unauthorized\r\nProxy-Authorization: Basic realm=\"proxy\"\r\n\r\n"
	hs200 = "HTTP/1.1 200 Connection established\r\n\r\n"
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
func StartHttpProxy(tcpConn *bufio.ReadWriter, handler AuthHandlerFunc,
	reader *bufio.Reader, writer *bufio.Writer) {

	fmt.Println("Start http proxy...")

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
			writer.Write([]byte(hs407))
			writer.Flush()

			return
		} else if !handler(user, passwd) {
			writer.Write([]byte(hs401))
			writer.Flush()

			return
		} else {
			writer.Write([]byte(hs200))
			writer.Flush()
		}
	} else if handler == nil {
		writer.Write([]byte(hs200))
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

	// fmt.Println("Leave http proxy with client:", tcpConn.RemoteAddr())
}
