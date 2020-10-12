package wsproxy

import (
	"bufio"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"io"
	"io/ioutil"
	"net"
	"net/url"

	"golang.org/x/net/websocket"
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
	reader *bufio.Reader, writer *bufio.Writer, server string) {
	defer tcpConn.Close()

	fmt.Println(ID, "Start proxy with client:", tcpConn.RemoteAddr())

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

	// 创建一个websocket的配置.
	config, err := websocket.NewConfig(server, server)
	if err != nil {
		fmt.Println(ID, "New client config error", err.Error())
	}
	// 设置Dialer为双栈模式, 以启用happyeballs.
	config.Dialer = &net.Dialer{
		DualStack: true,
	}

	// 设置tls相关参数.
	config.TlsConfig = &tls.Config{
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
	if config.TlsConfig.ServerName == "" {
		config.TlsConfig.ServerName = url.Hostname()
	}

	// 发起网络连接到url.
	fmt.Println(ID, "Connecting to:", url.Hostname(), "from", tcpConn.RemoteAddr())
	conn, err := config.Dialer.Dial("tcp", parseAuthority(url) /*"echo.websocket.org:443"*/)
	if err != nil {
		fmt.Println(ID, "Dialer error", err.Error())
		return
	}

	// 通过建立的网络连接配置tls, 然后发起握手.
	client := tls.Client(conn, config.TlsConfig)
	err = client.Handshake()
	if err != nil {
		fmt.Println(ID, "Handshake error", err.Error())
		return
	}

	// tls握手完成后得到tls.Conn, 使用它来创建websocket客户端对象, 返回时已完成websocket握手.
	ws, err := websocket.NewClient(config, client)
	if err != nil {
		fmt.Println(ID, "NewClient error", err.Error())
		client.Close()
		return
	}
	defer ws.Close()

	fmt.Println(ID, "Established with:", url.Hostname(), "from", tcpConn.RemoteAddr())

	// 开始使用ws对象收发websocket数据.
	errCh := make(chan error, 2)
	// origin -> ws
	go func(dst *websocket.Conn, src *bufio.Reader) {
		buf := make([]byte, 256*1024)
		var err error

		for {
			nr, er := src.Read(buf)
			if nr > 0 {
				nw, ew := dst.Write(buf[0:nr])
				if nw != nr {
					err = io.ErrShortWrite
					break
				}

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
	}(ws, reader)

	// ws -> origin
	go func(dst *bufio.Writer, src *websocket.Conn) {
		buf := make([]byte, 256*1024)
		var err error

		for {
			nr, er := src.Read(buf)
			if nr > 0 {
				nw, ew := dst.Write(buf[0:nr])
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
	}(writer, ws)

	// 等待转发退出.
	for i := 0; i < 2; i++ {
		e := <-errCh
		if e != nil {
			break
		}
	}

	fmt.Println(ID, "Exit proxy with client:", tcpConn.RemoteAddr())
}
