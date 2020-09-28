package wsproxy

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"io/ioutil"
	"log"
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
func StartConnectServer(server string) {
	pool := x509.NewCertPool()

	// 打开ca文件.
	ca, err := ioutil.ReadFile(CaCerts)
	if err == nil {
		pool.AppendCertsFromPEM(ca)
	} else if ServerVerifyClientCert {
		fmt.Println("Open ca file error", err.Error())
	}

	// 加载客户端证书文件及key.
	clientCert, err := tls.LoadX509KeyPair(ClientCert, ClientKey)
	if err != nil && ServerVerifyClientCert {
		fmt.Println("Open client cert file error", err.Error())
	}

	// 创建一个websocket的配置.
	config, err := websocket.NewConfig(server, server)
	if err != nil {
		fmt.Println("New client config error", err.Error())
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
		fmt.Println("Parse url error", err.Error())
		return
	}

	// 如果配置ServerName为空, 则添加一个默认hostname.
	if config.TlsConfig.ServerName == "" {
		config.TlsConfig.ServerName = url.Hostname()
	}

	// 发起网络连接到url.
	conn, err := config.Dialer.Dial("tcp", parseAuthority(url) /*"echo.websocket.org:443"*/)
	if err != nil {
		fmt.Println("Dialer error", err.Error())
		return
	}

	// 通过建立的网络连接配置tls, 然后发起握手.
	client := tls.Client(conn, config.TlsConfig)
	err = client.Handshake()
	if err != nil {
		fmt.Println("Handshake error", err.Error())
		return
	}

	// tls握手完成后得到tls.Conn, 使用它来创建websocket客户端对象, 返回时已完成websocket握手.
	ws, err := websocket.NewClient(config, client)
	if err != nil {
		fmt.Println("NewClient error", err.Error())
		client.Close()
		return
	}

	// 开始使用ws对象收发websocket数据.
	if _, err := ws.Write([]byte("hello, world!\n")); err != nil {
		log.Fatal(err)
	}
	var msg = make([]byte, 512)
	var n int
	if n, err = ws.Read(msg); err != nil {
		log.Fatal(err)
	}
	fmt.Printf("Received: %s.\n", msg[:n])
}
