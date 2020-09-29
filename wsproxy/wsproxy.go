package wsproxy

import (
	"bufio"
	"encoding/json"
	"fmt"
	"math/rand"
	"net"
	"os"
)

var (
	CaCerts = "/Users/jack/Downloads/cacert.pem" // ".wsporxy/certs/ca.crt"

	ServerCert = ".wsproxy/certs/server.crt"
	ServerKey  = ".wsproxy/certs/server.key"

	ClientCert = ".wsproxy/certs/client.crt"
	ClientKey  = ".wsproxy/certs/client.key"

	ServerVerifyClientCert = false
)

// Configuration ...
type Configuration struct {
	Servers                []string `json:"Servers"`
	ServerVerifyClientCert bool     `json:"VerifyClientCert"`
}

// Server ...
type Server struct {
	config Configuration
	listen *net.TCPListener
}

func (s *Server) handleClientConn(conn *net.TCPConn) {
	reader := bufio.NewReader(conn)
	peek, err := reader.Peek(1)
	if err != nil {
		fmt.Println("Peek first byte error", err.Error())
		return
	}

	writer := bufio.NewWriter(conn)

	if peek[0] == 0x05 {
		// 如果是socks5协议, 则调用socks5协议库, 若是client模式直接使用tls转发到服务器.
		numServer := len(s.config.Servers)
		if numServer > 0 {
			// 随机选择一个上游服务器用于转发socks5协议.
			idx := rand.Intn(numServer)
			StartConnectServer(conn, reader, writer, s.config.Servers[idx])
		} else {
			// 没有配置上游服务器地址, 直接作为socks5服务器提供socks5服务.
			StartSocks5Proxy(conn, reader, writer)
		}
	} else if peek[0] == 0x47 {
		// 如果'G', 则按http proxy处理, 若是client模式直接使用tls转发到服务器.
		fmt.Println("Http proxy protocol")
	} else if peek[0] == 0x16 {
		// 如果是tls协议, 则调用wss库处理socks协议, server处理tls加密的socks协议.
		fmt.Println("TLS protocol")
	} else {
		fmt.Println("Unknown protocol!")
	}
}

// NewServer ...
func NewServer(serverList []string) *Server {
	s := &Server{}

	// s.config.Servers = append(s.config.Servers, "wss://echo.websocket.org")

	file, err := os.Open("config.json")
	defer file.Close()
	if err != nil {
		fmt.Println("Configuration open error:", err)
		return s
	}

	configuration := Configuration{
		ServerVerifyClientCert: true,
	}

	decoder := json.NewDecoder(file)
	err = decoder.Decode(&configuration)
	if err != nil {
		fmt.Println("Configuration decode error:", err)
		return s
	}

	s.config = configuration
	ServerVerifyClientCert = configuration.ServerVerifyClientCert

	fmt.Println(s.config)

	return s
}

// Start start wserver...
func (s *Server) Start(addr string) error {
	tcpAddr, err := net.ResolveTCPAddr("tcp", addr)
	listen, err := net.ListenTCP("tcp", tcpAddr)
	if err != nil {
		return err
	}

	s.listen = listen

	for {
		c, err := s.listen.AcceptTCP()
		if err != nil {
			fmt.Println("Start Server, accept: ", err.Error())
			break
		}

		// start a new goroutine to handle the new connection.
		go s.handleClientConn(c)
	}

	return nil
}

// Stop stop socks5 server ...
func (s *Server) Stop() error {
	s.listen.Close()
	return nil
}
