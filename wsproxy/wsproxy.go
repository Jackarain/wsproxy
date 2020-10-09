package wsproxy

import (
	"bufio"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"math/rand"
	"net"
	"os"

	"git.superpool.io/Jackarain/wsporxy/websocket"
	"github.com/gobwas/ws"
)

var (
	// caCerts ...
	caCerts = ".wsproxy/certs/ca.crt"

	// ServerCert ...
	ServerCert = ".wsproxy/certs/server.crt"

	// ServerKey ...
	ServerKey = ".wsproxy/certs/server.key"

	// ClientCert ...
	ClientCert = ".wsproxy/certs/client.crt"

	// ClientKey ...
	ClientKey = ".wsproxy/certs/client.key"

	// UnixSockAddr ...
	UnixSockAddr = "/tmp/wsproxy.sock"

	// ServerVerifyClientCert ...
	ServerVerifyClientCert = false

	// ServerTLSConfig ...
	ServerTLSConfig *tls.Config
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
	config     Configuration
	listen     *net.TCPListener
	unixListen net.Listener

	authFunc AuthHandlerFunc
}

type bufferedConn struct {
	rw       *bufio.ReadWriter
	net.Conn // So that most methods are embedded
}

func newBufferedConn(c net.Conn) bufferedConn {
	return bufferedConn{bufio.NewReadWriter(bufio.NewReader(c), bufio.NewWriter(c)), c}
}

func newBufferedConnSize(c net.Conn, n int) bufferedConn {
	return bufferedConn{bufio.NewReadWriter(bufio.NewReaderSize(c, n), bufio.NewWriterSize(c, n)), c}
}

func (b bufferedConn) Peek(n int) ([]byte, error) {
	return b.rw.Peek(n)
}

func (b bufferedConn) Read(p []byte) (int, error) {
	return b.rw.Read(p)
}

func (s *Server) handleClientConn(conn *net.TCPConn) {
	bc := newBufferedConn(conn)
	defer bc.Close()

	reader := bc.rw.Reader
	peek, err := reader.Peek(1)
	if err != nil {
		fmt.Println("Peek first byte error", err.Error())
		return
	}

	writer := bc.rw.Writer

	idx := -1
	if len(s.config.Servers) > 0 {
		idx = rand.Intn(len(s.config.Servers))
	}

	if peek[0] == 0x05 {
		// 如果是socks5协议, 则调用socks5协议库, 若是client模式直接使用tls转发到服务器.
		if idx >= 0 {
			// 随机选择一个上游服务器用于转发socks5协议.
			StartConnectServer(conn, reader, writer, s.config.Servers[idx])
		} else {
			// 没有配置上游服务器地址, 直接作为socks5服务器提供socks5服务.
			StartSocks5Proxy(bc.rw, s.authFunc, reader, writer)
			fmt.Println("Leave socks5 proxy with client:", conn.RemoteAddr())
		}
	} else if peek[0] == 0x47 || peek[0] == 0x43 {
		// 如果'G' 或 'C', 则按http proxy处理, 若是client模式直接使用tls转发到服务器.
		if idx >= 0 {
			// 随机选择一个上游服务器用于转发http proxy协议.
			StartConnectServer(conn, reader, writer, s.config.Servers[idx])
		} else {
			StartHttpProxy(bc.rw, s.authFunc, reader, writer)
			fmt.Println("Leave http proxy with client:", conn.RemoteAddr())
		}
	} else if peek[0] == 0x16 {
		fmt.Println("Start tls connection...")

		// 转换成TLS connection对象.
		TLSConn := tls.Server(bc, ServerTLSConfig)

		// 开始握手.
		err := TLSConn.Handshake()
		if err != nil {
			fmt.Println("tls connection handshake fail", err.Error())
			return
		}

		// 创建websocket连接.
		wsconn, err := websocket.NewWebsocket(TLSConn)
		if err != nil {
			fmt.Println("tls connection Upgrade to websocket", err.Error())
			return
		}

		// 连接unix socket.
		c, err := net.Dial("unix", "/tmp/wsproxy.sock")
		if err != nil {
			fmt.Println("tls connect to unix socket", err.Error())
			return
		}

		errCh := make(chan error, 2)
		go func(c net.Conn, wsconn *websocket.Websocket) {
			buf := make([]byte, 32*1024)
			var err error

			for {
				nr, er := c.Read(buf)
				if nr > 0 {
					ew := wsconn.WriteMessage(ws.OpBinary, buf[0:nr])
					if ew != nil {
						err = ew
						break
					}
				} else {
					err = er
					break
				}
			}

			errCh <- err
		}(c, wsconn)

		go func(wsconn *websocket.Websocket, c net.Conn) {
			var err error
			for {
				_, msg, er := wsconn.ReadMessage()
				if len(msg) > 0 {
					nw, ew := c.Write(msg)
					if nw != len(msg) {
						err = ew
						break
					}
				} else {
					err = er
					break
				}
			}

			errCh <- err
		}(wsconn, c)

		for i := 0; i < 2; i++ {
			e := <-errCh
			if e != nil {
				break
			}
		}

		fmt.Println("Unix disconnect...")
	} else {
		fmt.Println("Unknown protocol!")
	}
}

func (s *Server) handleUnixConn(conn net.Conn) {
	bc := newBufferedConn(conn)
	defer bc.Close()
	reader := bc.rw.Reader
	peek, err := reader.Peek(1)
	if err != nil {
		return
	}

	writer := bc.rw.Writer

	if peek[0] == 0x05 {
		StartSocks5Proxy(bc.rw, s.authFunc, reader, writer)
	} else if peek[0] == 0x47 || peek[0] == 0x43 {
		StartHttpProxy(bc.rw, s.authFunc, reader, writer)
	} else {
		fmt.Println("Unknown protocol!")
	}
}

func initTLSServer() {
	// Server ca cert pool.
	CertPool := x509.NewCertPool()
	ca, err := ioutil.ReadFile(caCerts)
	if err == nil {
		CertPool.AppendCertsFromPEM(ca)
	} else if ServerVerifyClientCert {
		fmt.Println("Open ca file error", err.Error())
	}

	serverCert, err := tls.LoadX509KeyPair(ServerCert, ServerKey)
	if err != nil {
		fmt.Println("Open server cert file error", err.Error())
	}

	ServerTLSConfig = &tls.Config{
		MinVersion:   tls.VersionTLS13,
		RootCAs:      CertPool,
		Certificates: []tls.Certificate{serverCert},
		ServerName:   "Openvpn-server",
	}
}

// NewServer ...
func NewServer(serverList []string) *Server {
	// Init tls server.
	initTLSServer()

	// Make server.
	s := &Server{}

	// open config json file.
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
	go s.StartUnixSocket()
	return s.StartWithAuth(addr, nil)
}

// AuthHandleFunc ...
func (s *Server) AuthHandleFunc(handler func(string, string) bool) {
	s.authFunc = handler
}

// StartUnixSocket ...
func (s *Server) StartUnixSocket() error {
	if err := os.RemoveAll(UnixSockAddr); err != nil {
		log.Fatal(err)
	}

	listen, err := net.Listen("unix", UnixSockAddr)
	if err != nil {
		log.Fatal("listen error:", err)
	}

	s.unixListen = listen

	for {
		c, err := listen.Accept()
		if err != nil {
			fmt.Println("StartUnixSocket, accept: ", err.Error())
			break
		}

		go s.handleUnixConn(c)
	}

	return nil
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
			fmt.Println("StartWithAuth, accept: ", err.Error())
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
	s.unixListen.Close()
	return nil
}
