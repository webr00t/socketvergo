package main

import (
	"fmt"
	"io"
	"net"
	"strconv"
)

const (
	version5          = 0x05
	noAuthRequired    = 0x00
	cmdTCPConnect     = 0x01
	addrTypeIPv4      = 0x01
	addrTypeDomain    = 0x03
	addrTypeIPv6      = 0x04
	replySuccess      = 0x00
	replyGeneralError = 0x01
	rsv               = 0x00
)

// SocksConnector is socks network connector.
type SocksConnector struct {
}

func (s *SocksConnector) Connect(conn net.Conn, network string, addr string) (net.Conn, error) {
	if network != "tcp" && network != "tcp4" && network != "tcp6" {
		return conn, fmt.Errorf("%s is not supported", network)
	}
	saddr, err := ParseSocksAddr(addr)
	if err != nil {
		return conn, err
	}

	buf := make([]byte, 128)
	buf[0] = version5
	buf[1] = 1
	buf[2] = noAuthRequired

	if _, err = conn.Write(buf[0:3]); err != nil {
		return conn, err
	}

	var n int
	if n, err = io.ReadAtLeast(conn, buf, 2); err != nil {
		return conn, err
	}

	if n > 2 {
		err = fmt.Errorf("invalid data")
		return conn, err
	}

	if buf[0] != version5 {
		err = fmt.Errorf("unsupported socks version")
		return conn, err
	}

	if buf[1] != noAuthRequired {
		err = fmt.Errorf("unsupported auth method")
		return conn, err
	}

	buf[1] = cmdTCPConnect
	buf[2] = rsv
	buf[3] = saddr.addrType

	n = 4
	if saddr.addrType == addrTypeDomain {
		buf[n] = byte(len(saddr.addr))
		n++
	}
	copy(buf[n:n+len(saddr.addr)], saddr.addr)
	n += len(saddr.addr)
	buf[n] = byte(saddr.port >> 8)
	buf[n+1] = byte(saddr.port)
	n += 2

	if _, err = conn.Write(buf[:n]); err != nil {
		return conn, err
	}

	if n, err = io.ReadAtLeast(conn, buf, 5); err != nil {
		return conn, err
	}

	if buf[1] != replySuccess {
		err = fmt.Errorf("accept server errors")
		return conn, err
	}

	addrType := buf[3]
	var messageLen int
	switch addrType {
	case addrTypeIPv4:
		messageLen = 4 + 4 + 2
	case addrTypeIPv6:
		messageLen = 4 + 16 + 2
	case addrTypeDomain:
		messageLen = 5 + int(buf[4]) + 2
	default:
		err = fmt.Errorf("unsupported address type")
		return conn, err
	}

	if n < messageLen {
		var nn int
		nn, err = io.ReadAtLeast(conn, buf[n:], messageLen-n)
		n += nn
		if err != nil {
			return conn, err
		}
	}

	if n != messageLen {
		err = fmt.Errorf("invalid response data")
		return conn, err
	}

	return conn, err
}

type SocksAddr struct {
	addrType uint8
	addr     []byte
	port     uint16
}

func ParseSocksAddr(addr string) (*SocksAddr, error) {
	host, port, err := net.SplitHostPort(addr)
	if err != nil {
		return nil, err
	}

	portNum, err := parseSocksPort(port)
	if err != nil {
		return nil, err
	}

	addrType, ip, err := parseSocksHost(host)
	if err != nil {
		return nil, err
	}

	var addrBytes []byte
	if addrType == addrTypeDomain {
		if len(host) > 255 {
			return nil, fmt.Errorf("domain is too lang")
		}
		addrBytes = []byte(host)
	} else {
		addrBytes = ip
	}

	return &SocksAddr{
		addrType: addrType,
		addr:     addrBytes,
		port:     portNum,
	}, nil
}

func parseSocksPort(port string) (uint16, error) {
	if port == "" {
		return 0, fmt.Errorf("empty port")
	}
	var (
		portNum int
		err     error
	)
	if portNum, err = strconv.Atoi(port); err != nil {
		return 0, fmt.Errorf("invalid port: %v", port)
	}

	if 0 > portNum || portNum > 65535 {
		return 0, fmt.Errorf("invalid port: %v", portNum)
	}

	return uint16(portNum), nil
}

func parseSocksHost(host string) (addrType uint8, ip net.IP, err error) {
	if host == "" {
		return 0, nil, fmt.Errorf("empty host")
	}

	if ip := net.ParseIP(host); ip != nil {
		if v4 := ip.To4(); v4 != nil {
			return addrTypeIPv4, v4, nil
		}
		return addrTypeIPv6, ip, nil
	}
	return addrTypeDomain, nil, nil
}

// NewSocksDialer returns a socks5 dialer.
func NewSocksDialer(network string, addr string) Dialer {
	return NewDialer(network, addr, &SocksConnector{})
}
