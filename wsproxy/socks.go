package wsproxy

import (
	"bufio"
	"fmt"
	"io"
	"net"
)

const (
	socks5Version = uint8(5)

	socks5AuthNone         = uint8(0x00)
	socks5AuthGSSAPI       = uint8(0x01)
	socks5Auth             = uint8(0x02)
	socks5AuthUnAcceptable = uint8(0xFF)

	socks5CmdConnect = uint8(0x01)
	socks5CmdBind    = uint8(0x02)
	socks5CmdUDP     = uint8(0x03)

	socks5AtypIpv4       = uint8(0x01)
	socks5AtypDomainName = uint8(0x03)
	socks5AtypIpv6       = uint8(0x04)
)

type closeWriter interface {
	CloseWrite() error
}

func proxy(dst bufio.Writer, src io.Reader, errCh chan error) {
	buf := make([]byte, 32*1024)
	var err error

	for {
		nr, er := src.Read(buf)
		if nr > 0 {
			nw, ew := dst.Write(buf[0:nr])
			if nw != nr {
				err = ew
				break
			}
			dst.Flush()
		} else {
			err = er
			break
		}
	}

	dst.Flush()

	errCh <- err
}

func isIPv6(str string) bool {
	ip := net.ParseIP(str)
	return ip.To4() == nil
}

func isIPv4(str string) bool {
	ip := net.ParseIP(str)
	return ip.To4() != nil
}

func authMethod(handler AuthHandlerFunc, reader *bufio.Reader, writer *bufio.Writer) bool {
	defer writer.Flush()

	av, err := reader.ReadByte()
	if err != nil || av != 1 {
		fmt.Println("Socks5 auth version invalid")
		return false
	}

	uLen, err := reader.ReadByte()
	if err != nil || uLen <= 0 || uLen > 255 {
		fmt.Println("Socks5 auth user length invalid")
		return false
	}

	uBuf := make([]byte, uLen)
	nr, err := reader.Read(uBuf)
	if err != nil || nr != int(uLen) {
		fmt.Println("Socks5 auth user error", nr)
		return false
	}

	user := string(uBuf)

	pLen, err := reader.ReadByte()
	if err != nil || pLen <= 0 || pLen > 255 {
		fmt.Println("Socks5 auth passwd length invalid", pLen)
		return false
	}

	pBuf := make([]byte, pLen)
	nr, err = reader.Read(pBuf)
	if err != nil || nr != int(pLen) {
		fmt.Println("Socks5 auth passwd error", pLen, nr)
		return false
	}

	passwd := string(pBuf)

	// 执行认证操作, 认证通过.
	if handler != nil {
		if handler(user, passwd) {
			writer.WriteByte(0x01)
			writer.WriteByte(0x00)
			return true
		}

		// 认证失败.
		writer.WriteByte(0x01)
		writer.WriteByte(0x01)
	} else {
		writer.WriteByte(0x01)
		writer.WriteByte(0x00)
	}

	return false
}

// StartSocks5Proxy ...
func StartSocks5Proxy(tcpConn *bufio.ReadWriter, handler AuthHandlerFunc,
	reader *bufio.Reader, writer *bufio.Writer) {

	fmt.Println("Start socks5 proxy...")

	// |VER | NMETHODS | METHODS  |
	version, err := reader.ReadByte()
	if err != nil {
		fmt.Println("Socks5 version read error", err.Error())
		return
	}

	if version != socks5Version {
		fmt.Println("Socks5 version invalid", version)
		return
	}

	nmethods, err := reader.ReadByte()
	if err != nil {
		fmt.Println("Socks5 nmethods read error", err.Error())
		return
	}

	if nmethods < 0 || nmethods > 255 {
		fmt.Println("Socks5 nmethods invalid", nmethods)
		return
	}

	supportAuth := false
	method := socks5AuthNone
	for i := 0; i < int(nmethods); i++ {
		method, err = reader.ReadByte()
		if err != nil {
			fmt.Println("Socks5 methods read error", err.Error())
			return
		}
		if method == socks5Auth && handler != nil {
			supportAuth = true
		}
	}

	// |VER | METHOD |
	err = writer.WriteByte(version)
	if err != nil {
		fmt.Println("Socks5 write version error", err.Error())
		return
	}

	// 支持加密, 则回复加密方法.
	if supportAuth && handler != nil {
		method = socks5Auth
		err = writer.WriteByte(method)
		if err != nil {
			fmt.Println("Socks5 write socks5Auth error", err.Error())
			return
		}
	} else if handler == nil {
		// 服务器不支持加密, 直接通过.
		method = socks5AuthNone
		err = writer.WriteByte(method)
		if err != nil {
			fmt.Println("Socks5 write socks5AuthNone error", err.Error())
			return
		}
	} else {
		// 客户端不支持认证，服务器要求认证，返回socks5AuthUnAcceptable.
		method = socks5AuthUnAcceptable
		err = writer.WriteByte(method)
		if err != nil {
			fmt.Println("Socks5 write socks5AuthUnAcceptable error", err.Error())
			return
		}
	}
	writer.Flush()

	// Auth mode, read user passwd.
	if supportAuth {
		if !authMethod(handler, reader, writer) {
			fmt.Println("Socks5 auth not passed")
			return
		}
	}

	// |VER | CMD |  RSV  | ATYP | DST.ADDR | DST.PORT |

	// 认证通过或非认证模式.
	handshakeVersion, err := reader.ReadByte()
	if err != nil || handshakeVersion != socks5Version {
		if err != nil {
			fmt.Println("Socks5 read handshake version error", err.Error())
		}
		return
	}

	command, err := reader.ReadByte()
	if err != nil {
		fmt.Println("Socks5 read command error", err.Error())
		return
	}
	if command != socks5CmdConnect &&
		command != socks5CmdBind &&
		command != socks5CmdUDP {
		fmt.Println("Socks5 read command invalid", command)
		return
	}

	reader.ReadByte() // rsv byte
	atyp, err := reader.ReadByte()
	if err != nil {
		fmt.Println("Socks5 read atyp error", err.Error())
		return
	}
	if atyp != socks5AtypDomainName &&
		atyp != socks5AtypIpv4 &&
		atyp != socks5AtypIpv6 {
		fmt.Println("Socks5 read atyp invalid", atyp)
		return
	}

	hostname := ""
	switch {
	case atyp == socks5AtypIpv4:
		{
			IPv4Buf := make([]byte, 4)
			nr, err := reader.Read(IPv4Buf)
			if err != nil || nr != 4 {
				fmt.Println("Socks5 read atyp ipv4 address error")
				return
			}

			ip := net.IP(IPv4Buf)
			hostname = ip.String()
		}
	case atyp == socks5AtypIpv6:
		{
			IPv6Buf := make([]byte, 16)
			nr, err := reader.Read(IPv6Buf)
			if err != nil || nr != 16 {
				fmt.Println("Socks5 read atyp ipv6 address error")
				return
			}

			ip := net.IP(IPv6Buf)
			hostname = ip.String()
		}
	case atyp == socks5AtypDomainName:
		{
			dnLen, err := reader.ReadByte()
			if err != nil || int(dnLen) < 0 {
				fmt.Println("Socks5 read domain len error", dnLen)
				return
			}

			domain := make([]byte, dnLen)
			nr, err := reader.Read(domain)
			if err != nil || nr != int(dnLen) {
				fmt.Println("Socks5 read atyp domain error", domain)
				return
			}

			hostname = string(domain)
		}
	}

	portNum1, err := reader.ReadByte()
	if err != nil {
		fmt.Println("Socks5 read atyp port byte1 error")
		return
	}

	portNum2, err := reader.ReadByte()
	if err != nil {
		fmt.Println("Socks5 read atyp port byte2 error")
		return
	}

	port := uint16(portNum1)<<8 + uint16(portNum2)
	hostname = fmt.Sprintf("%s:%d", hostname, port)

	//  |VER | REP |  RSV  | ATYP | BND.ADDR | BND.PORT |
	writer.WriteByte(socks5Version)

	// Start connect to target host.
	targetConn, err := net.Dial("tcp", hostname)
	if err != nil {
		writer.WriteByte(1) // SOCKS5_GENERAL_SOCKS_SERVER_FAILURE
	} else {
		writer.WriteByte(0) // SOCKS5_SUCCEEDED
	}

	// rsv byte
	writer.WriteByte(0)

	hostport := targetConn.RemoteAddr().String()
	host, _, _ := net.SplitHostPort(hostport)
	if isIPv4(host) {
		writer.WriteByte(socks5AtypIpv4)
		writer.Write(net.ParseIP(host).To4())
	} else if isIPv6(host) {
		writer.WriteByte(socks5AtypIpv6)
		writer.Write(net.ParseIP(host).To16())
	} else {
		writer.WriteByte(socks5AtypDomainName)
		writer.WriteByte(byte(len(hostname)))
		writer.WriteString(hostname)
	}

	writer.WriteByte(portNum1)
	writer.WriteByte(portNum2)

	writer.Flush()

	// Start proxying
	errCh := make(chan error, 2)
	tw := bufio.NewWriter(targetConn)
	go proxy(*tw, tcpConn.Reader, errCh)
	go proxy(*tcpConn.Writer, targetConn, errCh)

	// Wait
	for i := 0; i < 2; i++ {
		e := <-errCh
		if e != nil {
			break
		}
	}
}
