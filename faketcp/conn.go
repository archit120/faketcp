package faketcp

import (
	"fmt"
	"io"
	"net"
	"syscall"
	"time"

	"github.com/archit120/ethernet-go/header"
)

var CONNCHANBUFSIZE = 1024
var CONNTIMEOUT = 60

const (
	CONNECTING = iota
	CONNECTED
	CLOSING
	CLOSED
)

type Conn struct {
	localAddress  [4]byte
	localPort     int
	remoteAddress [4]byte
	remotePort    int
	State         int
	fd            int
	nextSYN       int
	nextACK       int
	LastUpdate    time.Time
}

func NewConn(localAddr [4]byte, remoteAddr [4]byte, state int, fd int) *Conn {
	conn := &Conn{
		localAddress:  localAddr,
		remoteAddress: remoteAddr,
		fd:			   fd,
		State:         state,
		LastUpdate:    time.Now(),
		nextSYN:       1,
	}
	go conn.keepAlive()
	return conn
}

func (conn *Conn) UpdateTime() {
	conn.LastUpdate = time.Now()
}

func (conn *Conn) IsTimeout() bool {
	now := time.Now()
	return now.Sub(conn.LastUpdate) > time.Second*time.Duration(CONNTIMEOUT)
}

func (conn *Conn) keepAlive() {
	for {
		if conn.State == CLOSED || conn.State == CLOSING {
			return

		} else if conn.State == CONNECTED {
			ipHeader, tcpHeader := header.BuildTcpHeader(conn.LocalAddr().String(), conn.RemoteAddr().String())
			tcpHeader.Flags = header.ACK
			tcpHeader.Ack = 1
			tcpHeader.Seq = 1

			packet := header.BuildTcpPacket(ipHeader, tcpHeader, []byte{})
			conn.WriteWithHeader(packet)
		}
		time.Sleep(time.Second)
	}
}

//Block
func (conn *Conn) Read(b []byte) (n int, err error) {
	defer func() {
		if r := recover(); r != nil {
			n, err = -1, io.EOF
		}
	}()
	if conn.State != CONNECTED {
		return -1, io.EOF
	}

	for {
		s, ok := <-conn.InputChan
		if !ok {
			return -1, io.EOF
		}

		_, _, _, _, data, _ := header.Get([]byte(s))
		ls, ln := len(data), len(b)
		if ls < 0 {
			continue
		}

		l := ls
		if ln < ls {
			l = ln
		}
		for i := 0; i < l; i++ {
			b[i] = data[i]
		}
		return ls, nil
	}
}

//Block
func (conn *Conn) Write(b []byte) (n int, err error) {

	ipHeader, tcpHeader := header.BuildTcpHeader(conn.LocalAddr().String(), conn.RemoteAddr().String())
	tcpHeader.Flags = 0x18
	tcpHeader.Ack = 1
	tcpHeader.Seq = 1

	packet := header.BuildTcpPacket(ipHeader, tcpHeader, b)
	return conn.WriteWithHeader(packet)
}

//NoBlock
func (conn *Conn) ReadWithHeader(b []byte) (n int, err error) {
	
	n, from, err := syscall.Recvfrom(conn.fd, b, 0)
	for ; err == nil && from.(*syscall.SockaddrInet4).Addr != conn.remoteAddress ;{
		n, from, err = syscall.Recvfrom(conn.fd, b, 0)
	}

	return n, err
}

//NoBlock
func (conn *Conn) WriteWithHeader(b []byte) (n int, err error) {
	to := syscall.SockaddrInet4{
		Port: conn.remotePort,
		Addr: conn.remoteAddress,
	} 
	return len(b), syscall.Sendto(conn.fd, b, 0, to)
}

func (conn *Conn) CloseRequest() (err error) {
	if conn.State != CONNECTED {
		return nil
	}

	defer func() {
		conn.State = CLOSED
	}()

	conn.State = CLOSING
	ipHeader, tcpHeader := header.BuildTcpHeader(conn.LocalAddr().String(), conn.RemoteAddr().String())
	tcpHeader.Seq = 1
	tcpHeader.Ack = 1
	tcpHeader.Flags = header.FIN
	packet := header.BuildTcpPacket(ipHeader, tcpHeader, []byte{})

	done := make(chan int)
	go func() {
		for i := 0; i < RETRYTIME; i++ {
			select {
			case <-done:
				return
			default:
			}
			conn.WriteWithHeader(packet)
			time.Sleep(time.Millisecond * RETRYINTERVAL)
		}
	}()

	after := time.After(time.Millisecond * RETRYINTERVAL * RETRYTIME)
	buf := make([]byte, BUFFERSIZE)
	timeOut := false
	for !timeOut {
		if n, err := conn.ReadWithHeader(buf); n > 0 && err == nil {
			_, _, _, tcpHeader, _, _ := header.Get(buf[:n])
			if tcpHeader.Flags == (header.ACK|header.FIN) && tcpHeader.Ack == 1 {
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

	ipHeader, tcpHeader = header.BuildTcpHeader(conn.LocalAddr().String(), conn.RemoteAddr().String())
	tcpHeader.Seq = 1
	tcpHeader.Ack = 1
	tcpHeader.Flags = header.ACK
	packet = header.BuildTcpPacket(ipHeader, tcpHeader, []byte{})
	conn.WriteWithHeader(packet)

	return nil
}

func (conn *Conn) CloseResponse() (err error) {
	if conn.State != CONNECTED {
		return nil
	}

	defer func() {
		conn.State = CLOSED
		conn.Close()
	}()
	conn.State = CLOSING

	ipHeader, tcpHeader := header.BuildTcpHeader(conn.LocalAddr().String(), conn.RemoteAddr().String())
	tcpHeader.Seq = 1
	tcpHeader.Ack = 1
	tcpHeader.Flags = header.FIN | header.ACK
	packet := header.BuildTcpPacket(ipHeader, tcpHeader, []byte{})

	done := make(chan int)
	go func() {
		for i := 0; i < RETRYTIME; i++ {
			select {
			case <-done:
				return
			default:
			}
			conn.WriteWithHeader(packet)
			time.Sleep(time.Millisecond * RETRYINTERVAL)
		}
	}()

	after := time.After(time.Millisecond * RETRYINTERVAL * RETRYTIME)
	buf := make([]byte, BUFFERSIZE)
	timeOut := false
	for !timeOut {
		if n, err := conn.ReadWithHeader(buf); n > 0 && err == nil {
			_, _, _, tcpHeader, _, _ := header.Get(buf[:n])
			if tcpHeader.Flags == header.ACK && tcpHeader.Ack == 1 {
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
	return err
}

func (conn *Conn) Close() error {
	conn.CloseRequest()
	key := conn.LocalAddr().String() + ":" + conn.RemoteAddr().String()
	faketcpServer.CloseConn(key)

	go func() {
		defer func() {
			recover()
		}()
		close(conn.InputChan)
	}()
	go func() {
		defer func() {
			recover()
		}()
		close(conn.OutputChan)
	}()
	return nil
}

func (conn *Conn) LocalAddr() net.Addr {
	return conn.localAddress
}

func (conn *Conn) RemoteAddr() net.Addr {
	return conn.remoteAddress
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
