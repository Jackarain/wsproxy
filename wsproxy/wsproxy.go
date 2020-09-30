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
	caCerts = "/Users/jack/Downloads/cacert.pem" // ".wsporxy/certs/ca.crt"

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

// AuthHandlerFunc ...
type AuthHandlerFunc func(string, string) bool

// AuthHander interface ...
type AuthHander interface {
	Auth(string, string) bool
}

// Server ...
type Server struct {
	config   Configuration
	listen   *net.TCPListener
	authFunc AuthHandlerFunc
}

func (s *Server) handleClientConn(conn *net.TCPConn) {
	reader := bufio.NewReader(conn)
	peek, err := reader.Peek(1)
	if err != nil {
		fmt.Println("Peek first byte error", err.Error())
		return
	}

	writer := bufio.NewWriter(conn)

	idx := -1
	if len(s.config.Servers) > 0 {
		idx = rand.Intn(len(s.config.Servers))
	}

	if peek[0] == 0x05 {
		// 如果是socks5协议, 则调用socks5协议库, 若是client模式直接使用tls转发到服务器.
		if idx > 0 {
			// 随机选择一个上游服务器用于转发socks5协议.
			StartConnectServer(conn, reader, writer, s.config.Servers[idx])
		} else {
			// 没有配置上游服务器地址, 直接作为socks5服务器提供socks5服务.
			StartSocks5Proxy(conn, s.authFunc, reader, writer)
		}
	} else if peek[0] == 0x47 || peek[0] == 0x43 {
		// 如果'G' 或 'C', 则按http proxy处理, 若是client模式直接使用tls转发到服务器.
		if idx > 0 {
			// 随机选择一个上游服务器用于转发http proxy协议.
			StartConnectServer(conn, reader, writer, s.config.Servers[idx])
		} else {
			StartHttpProxy(conn, s.authFunc, reader, writer)
		}
	} else if peek[0] == 0x16 {
		// 如果是tls协议, 则调用wss库处理socks5/http proxy协议, server处理tls加密的socks5/http proxy协议.
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
	return s.StartWithAuth(addr, nil)
}

// AuthHandleFunc ...
func (s *Server) AuthHandleFunc(handler func(string, string) bool) {
	s.authFunc = handler
}

// StartWithAuth start wserver...
func (s *Server) StartWithAuth(addr string, handler AuthHander) error {
	tcpAddr, err := net.ResolveTCPAddr("tcp", addr)
	listen, err := net.ListenTCP("tcp", tcpAddr)
	if err != nil {
		return err
	}

	s.listen = listen
	if handler != nil {
		s.authFunc = handler.Auth
	}

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
