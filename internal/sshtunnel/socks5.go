package sshtunnel

import (
	"encoding/binary"
	"fmt"
	"io"
	"net"
	"strconv"
)

const (
	socks5Version   = 0x05
	socks5NoAuth    = 0x00
	socks5CmdConnect = 0x01
	socks5AddrIPv4   = 0x01
	socks5AddrDomain = 0x03
	socks5AddrIPv6   = 0x04
)

// socks5Handshake performs the SOCKS5 handshake and returns the target address.
func socks5Handshake(conn net.Conn) (string, error) {
	// Version + number of methods
	buf := make([]byte, 2)
	if _, err := io.ReadFull(conn, buf); err != nil {
		return "", fmt.Errorf("read version: %w", err)
	}
	if buf[0] != socks5Version {
		return "", fmt.Errorf("unsupported SOCKS version: %d", buf[0])
	}

	// Read methods
	methods := make([]byte, buf[1])
	if _, err := io.ReadFull(conn, methods); err != nil {
		return "", fmt.Errorf("read methods: %w", err)
	}

	// Reply: no auth required
	if _, err := conn.Write([]byte{socks5Version, socks5NoAuth}); err != nil {
		return "", fmt.Errorf("write auth reply: %w", err)
	}

	// Read connect request: VER CMD RSV ATYP
	header := make([]byte, 4)
	if _, err := io.ReadFull(conn, header); err != nil {
		return "", fmt.Errorf("read request header: %w", err)
	}
	if header[0] != socks5Version {
		return "", fmt.Errorf("invalid request version: %d", header[0])
	}
	if header[1] != socks5CmdConnect {
		socks5Reply(conn, 0x07) // command not supported
		return "", fmt.Errorf("unsupported command: %d", header[1])
	}

	// Parse destination address
	var host string
	switch header[3] {
	case socks5AddrIPv4:
		addr := make([]byte, 4)
		if _, err := io.ReadFull(conn, addr); err != nil {
			return "", fmt.Errorf("read IPv4 addr: %w", err)
		}
		host = net.IP(addr).String()
	case socks5AddrDomain:
		lenBuf := make([]byte, 1)
		if _, err := io.ReadFull(conn, lenBuf); err != nil {
			return "", fmt.Errorf("read domain length: %w", err)
		}
		domain := make([]byte, lenBuf[0])
		if _, err := io.ReadFull(conn, domain); err != nil {
			return "", fmt.Errorf("read domain: %w", err)
		}
		host = string(domain)
	case socks5AddrIPv6:
		addr := make([]byte, 16)
		if _, err := io.ReadFull(conn, addr); err != nil {
			return "", fmt.Errorf("read IPv6 addr: %w", err)
		}
		host = net.IP(addr).String()
	default:
		socks5Reply(conn, 0x08) // address type not supported
		return "", fmt.Errorf("unsupported address type: %d", header[3])
	}

	// Read port (2 bytes, big endian)
	portBuf := make([]byte, 2)
	if _, err := io.ReadFull(conn, portBuf); err != nil {
		return "", fmt.Errorf("read port: %w", err)
	}
	port := binary.BigEndian.Uint16(portBuf)

	return net.JoinHostPort(host, strconv.Itoa(int(port))), nil
}

// socks5Reply sends a SOCKS5 reply.
func socks5Reply(conn net.Conn, status byte) {
	// VER REP RSV ATYP BND.ADDR BND.PORT
	reply := []byte{socks5Version, status, 0x00, socks5AddrIPv4, 0, 0, 0, 0, 0, 0}
	conn.Write(reply)
}
