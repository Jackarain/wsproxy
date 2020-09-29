package wsproxy

import (
	"bufio"
	"fmt"
	"net"
)

const (
	socks5Version5 = uint8(5)

	socks5AuthNone         = uint8(0x00)
	socks5Auth             = uint8(0x02)
	socks5AuthUnacceptable = uint8(0xFF)
)

// StartSocks5Proxy ...
func StartSocks5Proxy(tcpConn *net.TCPConn,
	reader *bufio.Reader, writer *bufio.Writer) {

	version, err := reader.ReadByte()
	if err != nil {
		fmt.Println("Socks5 version read error", err.Error())
		return
	}

	if version != Socks5Version {
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
	for i := 0; i < nmethods; i++ {
		status, err := reader.ReadByte()
		if err != nil {
			fmt.Println("Socks5 methods read error", err.Error())
			return
		}
		if status == socks5Auth {
			supportAuth = true
		}
	}

	if supportAuth {
		err := writer.WriteByte(version)
		if err != nil {
			fmt.Println("Socks5 write version error", err.Error())
			return
		}
	}

}
