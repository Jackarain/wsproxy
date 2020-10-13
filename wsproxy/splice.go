package wsproxy

import (
	"bufio"
	"bytes"
	"compress/zlib"
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"io"
	"io/ioutil"
	"net"
	"net/url"

	"gitee.com/jackarain/wsproxy/websocket"
	"github.com/gobwas/ws"
)

var portMap = map[string]string{
	"ws":  "80",
	"wss": "443",
}

func parseAuthority(location *url.URL) string {
	if _, ok := portMap[location.Scheme]; ok {
		if _, _, err := net.SplitHostPort(location.Host); err != nil {
			return net.JoinHostPort(location.Host, portMap[location.Scheme])
		}
	}
	return location.Host
}

// StartConnectServer ...
func StartConnectServer(ID uint64, tcpConn *net.TCPConn,
	reader *bufio.Reader, writer *bufio.Writer, server string) (insize, tosize int) {
	defer tcpConn.Close()

	insize = 0
	tosize = 0

	fmt.Println(ID, "* Start proxy with client:", tcpConn.RemoteAddr())

	// 打开ca文件.
	pool := x509.NewCertPool()
	ca, err := ioutil.ReadFile(caCerts)
	if err == nil {
		pool.AppendCertsFromPEM(ca)
	} else if ServerVerifyClientCert {
		fmt.Println(ID, "Open ca file error", err.Error())
	}

	// 加载客户端证书文件及key.
	clientCert, err := tls.LoadX509KeyPair(ClientCert, ClientKey)
	if err != nil && ServerVerifyClientCert {
		fmt.Println(ID, "Open client cert file error", err.Error())
	}

	// 设置tls相关参数.
	tlsConfig := &tls.Config{
		RootCAs:            pool,
		Certificates:       []tls.Certificate{clientCert},
		InsecureSkipVerify: !ServerVerifyClientCert,
	}

	// 解析url.
	url, err := url.Parse(server)
	if err != nil {
		fmt.Println(ID, "Parse url error", err.Error())
		return
	}

	// 如果配置ServerName为空, 则添加一个默认hostname.
	if tlsConfig.ServerName == "" {
		tlsConfig.ServerName = url.Hostname()
	}

	// 发起网络连接到url.
	fmt.Println(ID, "Connecting to:", url.Hostname(), "from", tcpConn.RemoteAddr())

	var header ws.HandshakeHeader
	if Encoding == "zlib" {
		header = ws.HandshakeHeaderString("Content-Encoding: zlib\r\n")
	}
	d := ws.Dialer{
		TLSConfig: tlsConfig,
		Header:    header,
	}

	c, _, _, err := d.Dial(context.Background(), server)
	if err != nil {
		fmt.Println(ID, "Dialer error", err.Error())
		return
	}

	defer c.Close()

	rw := c.(io.ReadWriter)
	conn := &websocket.Websocket{
		Conn: &rw,
	}

	fmt.Println(ID, "Established with:", url.Hostname(), "from", tcpConn.RemoteAddr())

	// 开始使用ws对象收发websocket数据.
	errCh := make(chan error, 2)
	// origin -> ws
	go func(dst *websocket.Websocket, src *bufio.Reader) {
		buf := make([]byte, 256*1024)
		var err error
		var sbuf []byte

		for {
			nr, er := src.Read(buf)
			sbuf = buf

			if nr > 0 {
				if Encoding == "zlib" {
					var gbuf bytes.Buffer
					w := zlib.NewWriter(&gbuf)
					nz, ez := w.Write(buf[0:nr])
					if nz != nr {
						err = io.ErrShortWrite
						break
					}
					if ez != nil {
						err = ez
						break
					}
					w.Close()

					sbuf = gbuf.Bytes()
					nr = len(sbuf)
					tosize = tosize + (nz - nr)
				} else {
					tosize = tosize + nr
				}

				ew := dst.WriteMessage(ws.OpBinary, sbuf[0:nr])
				if ew != nil {
					err = ew
					break
				}
			}

			if er != nil {
				err = er
				break
			}
		}

		errCh <- err
	}(conn, reader)

	// ws -> origin
	go func(dst *bufio.Writer, src *websocket.Websocket) {
		sbuf := make([]byte, 512*1024)
		var err error

		for {
			_, buf, er := src.ReadMessage()
			nr := len(buf)
			if nr > 0 {
				if Encoding == "zlib" {
					b := bytes.NewReader(buf[0:nr])
					r, ez := zlib.NewReader(b)
					if ez != nil {
						err = ez
						break
					}
					nn, ez := r.Read(sbuf)
					if ez != nil && ez != io.EOF {
						er = ez
						if nn <= 0 {
							err = ez
							break
						}
					}
					insize = insize + (nn - nr)
					nr = nn
					r.Close()
				} else {
					sbuf = buf
					insize = insize + nr
				}

				nw, ew := dst.Write(sbuf[0:nr])
				if nw != nr {
					err = io.ErrShortWrite
					break
				}
				dst.Flush()

				if ew != nil {
					err = ew
					break
				}
			}

			if er != nil {
				err = er
				break
			}
		}

		dst.Flush()
		errCh <- err
	}(writer, conn)

	// 等待转发退出.
	for i := 0; i < 2; i++ {
		e := <-errCh
		if e != nil {
			break
		}
	}

	return
}
