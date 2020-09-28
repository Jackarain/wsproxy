package socks

import (
	"fmt"
	"net"
	"sync"
)

// onAccept 每当有连接上触发调用，返回false将拒绝连接.
type onAccept func(addr net.Addr) bool

// onAuth 调用认证的时候，返回false时将拒绝服务.
type onAuth func(user, passwd string, addr net.Addr) bool

// onDisconnect 连接断开时触发.
type onDisconnect func(user string, addr net.Addr)

// onForward 每当客户端发起转发请求的时候触发.
type onForward func(user string, target net.Addr)

// onForwardError 每当客户端发起转发请求失败或出错的时候触发.
type onForwardError func(user string, target net.Addr)

// Socks5Server ...
type Socks5Server struct {
	listen       *net.TCPListener
	accept       onAccept
	auth         onAuth
	disconnect   onDisconnect
	forward      onForward
	forwardError onForwardError

	totalTraffic sync.Map
	clientConn   []*net.TCPConn
}

func (s *Socks5Server) handleClientConn(c *net.TCPConn) {
	var b []byte
	c.Read(b)
}

// NewSocks5Server ...
func NewSocks5Server() (*Socks5Server, error) {
	s := &Socks5Server{}
	return s, nil
}

// Start start socks5 server...
func (s *Socks5Server) Start(addr string) error {
	tcpAddr, err := net.ResolveTCPAddr("tcp", addr)
	listen, err := net.ListenTCP("tcp", tcpAddr)
	if err != nil {
		return err
	}

	s.listen = listen

	for {
		c, err := s.listen.AcceptTCP()
		if err != nil {
			fmt.Println("StartSocks5Server, accept: ", err.Error())
			break
		}

		s.clientConn = append(s.clientConn, c)

		// start a new goroutine to handle the new connection.
		go s.handleClientConn(c)
	}

	return nil
}

// Stop stop socks5 server ...
func (s *Socks5Server) Stop() error {
	for _, conn := range s.clientConn {
		conn.Close()
	}

	s.listen.Close()
	return nil
}

// FetchTraffic fetch traffic...
func (s *Socks5Server) FetchTraffic(user string) (value interface{}, ok bool) {
	return s.totalTraffic.Load(user)
}
