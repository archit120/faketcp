package faketcp

import (
	"encoding/binary"
	"fmt"
	"net"
	"syscall"
	"time"

	"github.com/archit120/faketcp/header"
	"github.com/archit120/faketcp/netinfo"
)

const (
	CONNECTING = iota
	CONNECTED
	CLOSING
	CLOSED
)

type PacketConn struct {
	localAddress  uint32
	localPort     int
	State         int
	fd            int
	nextSEQ       int
	nextACK       int
	LastUpdate    time.Time
}

func NewPacketConn(localAddr uint32, localPort int, state int, fd int) *PacketConn {
	conn := &PacketConn{
		localPort:     localPort,
		localAddress:  localAddr,
		fd:            fd,
		State:         state,
		LastUpdate:    time.Now(),
		nextSEQ:       1,
	}
	go conn.keepAlive()
	return conn
}

//Block needs upto 40 bytes extra :(
func (conn *Conn) Read(b []byte) (n int, err error) {
	n, err = conn.ReadWithHeader(b)
	if err != nil {
		return 0, err
	}
	copy(b, b[20:])
	return n - 20, err
}

//Block
func (conn *Conn) Write(b []byte) (n int, err error) {

	return conn.WriteWithHeader(header.BuildTcpPacket(conn.localAddress, uint16(conn.localPort), conn.remoteAddress,
		uint16(conn.remotePort), uint32(conn.nextSEQ), uint32(conn.nextACK), header.SYN|header.ACK, b))
}

//Blocks and Reads
func (conn *Conn) ReadWithHeader(b []byte) (n int, err error) {

	n, from, err := syscall.Recvfrom(conn.fd, b, 0)
	ip, _ := netinfo.B2ip(from.(*syscall.SockaddrInet4).Addr[:])
	var hdr header.TCP
	hdr.Unmarshal(b[20:])
	for err == nil && ip != conn.remoteAddress && hdr.DstPort != uint16(conn.localPort) {
		// Also verify port and checksum !!!
		// Maybe can ignore checksum
		n, from, err = syscall.Recvfrom(conn.fd, b, 0)
		ip, _ = netinfo.B2ip(from.(*syscall.SockaddrInet4).Addr[:])
		hdr.Unmarshal(b[20:])
	}
	if err != nil {
		return 0, err
	}
	n = copy(b, b[20:n])
	conn.nextACK = int(hdr.Seq) + n + int(hdr.Flags&header.SYN)
	return n, err
}

//B
func (conn *Conn) WriteWithHeader(b []byte) (n int, err error) {
	var remoteadd [4]byte
	binary.BigEndian.PutUint32(remoteadd[:], conn.remoteAddress)
	to := syscall.SockaddrInet4{
		Port: conn.remotePort,
		Addr: remoteadd,
	}
	conn.nextSEQ += len(b)-20
	return len(b), syscall.Sendto(conn.fd, b, 0, &to)
}

func (conn *Conn) CloseRequest() (err error) {
	if conn.State != CONNECTED {
		return nil
	}

	defer func() {
		conn.State = CLOSED
	}()

	conn.State = CLOSING
	tcpPacket := header.BuildTcpPacket(conn.localAddress, uint16(conn.localPort), conn.remoteAddress,
		uint16(conn.remotePort), uint32(conn.nextSEQ), uint32(conn.nextACK), header.FIN, []byte{})

	done := make(chan int)
	go func() {
		for i := 0; i < RETRYTIME; i++ {
			select {
			case <-done:
				return
			default:
			}
			conn.WriteWithHeader(tcpPacket)
			time.Sleep(time.Millisecond * RETRYINTERVAL)
		}
	}()

	after := time.After(time.Millisecond * RETRYINTERVAL * RETRYTIME)
	buf := make([]byte, BUFFERSIZE)
	timeOut := false
	for !timeOut {
		if n, err := conn.ReadWithHeader(buf); n > 0 && err == nil {
			var hdr header.TCP
			hdr.Unmarshal(buf)
			if hdr.Flags == (header.ACK | header.FIN) {
				close(done)
				break
			}
		}

		select {
		case <-after:
			err = fmt.Errorf("timeout")
			timeOut = true
		default:
		}
	}

	if err != nil {
		return err
	}

	tcpPacket = header.BuildTcpPacket(conn.localAddress, uint16(conn.localPort), conn.remoteAddress,
		uint16(conn.remotePort), uint32(conn.nextSEQ), uint32(conn.nextACK), header.ACK, []byte{})
	conn.WriteWithHeader(tcpPacket)

	return nil
}

// func (conn *Conn) CloseResponse() (err error) {
// 	if conn.State != CONNECTED {
// 		return nil
// 	}

// 	defer func() {
// 		conn.State = CLOSED
// 		conn.Close()
// 	}()
// 	conn.State = CLOSING

// 	ipHeader, tcpHeader := header.BuildTcpHeader(conn.LocalAddr().String(), conn.RemoteAddr().String())
// 	tcpHeader.Seq = 1
// 	tcpHeader.Ack = 1
// 	tcpHeader.Flags = header.FIN | header.ACK
// 	packet := header.BuildTcpPacket(ipHeader, tcpHeader, []byte{})

// 	done := make(chan int)
// 	go func() {
// 		for i := 0; i < RETRYTIME; i++ {
// 			select {
// 			case <-done:
// 				return
// 			default:
// 			}
// 			conn.WriteWithHeader(packet)
// 			time.Sleep(time.Millisecond * RETRYINTERVAL)
// 		}
// 	}()

// 	after := time.After(time.Millisecond * RETRYINTERVAL * RETRYTIME)
// 	buf := make([]byte, BUFFERSIZE)
// 	timeOut := false
// 	for !timeOut {
// 		if n, err := conn.ReadWithHeader(buf); n > 0 && err == nil {
// 			_, _, _, tcpHeader, _, _ := header.Get(buf[:n])
// 			if tcpHeader.Flags == header.ACK && tcpHeader.Ack == 1 {
// 				close(done)
// 				break
// 			}
// 		}

// 		select {
// 		case <-after:
// 			err = fmt.Errorf("timeout")
// 			timeOut = true
// 		default:
// 		}
// 	}
// 	return err
// }

func (conn *Conn) Close() error {
	conn.CloseRequest()
	return nil
}
func (conn *Conn) SetDeadline(t time.Time) error {
	return nil
}

func (conn *Conn) SetReadDeadline(t time.Time) error {
	return nil
}

func (conn *Conn) SetWriteDeadline(t time.Time) error {
	return nil
}


// TODO: implement
func (conn *Conn) LocalAddr() net.Addr {
	return &net.UDPAddr{

	}
}

func (conn *Conn) RemoteAddr() net.Addr {
	return &net.UDPAddr{
		
	}
}