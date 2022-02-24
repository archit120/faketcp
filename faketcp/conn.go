package faketcp

import (
	// "encoding/binary"
	"fmt"
	"net"
	"time"

	"github.com/archit120/faketcp/header"
	"github.com/archit120/faketcp/netinfo"
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
	localAddress  uint32
	localPort     int
	remoteAddress uint32
	remotePort    int
	State         int
	internalConn  *net.IPConn
	nextSEQ       int
	nextACK       int
	LastUpdate    time.Time
}

func NewConn(internalConn *net.IPConn, localPort int, remoteAddr uint32, remotePort int, state int) *Conn {
	laddr, _ := netinfo.B2ip(internalConn.LocalAddr().(*net.IPAddr).IP)
	conn := &Conn{
		localPort:     localPort,
		localAddress:  laddr,
		remotePort:    remotePort,
		remoteAddress: remoteAddr,
		internalConn:  internalConn,
		State:         state,
		LastUpdate:    time.Now(),
		nextSEQ:       1,
		nextACK:       0,
	}
	go conn.keepAlive()
	return conn
}

func (conn *Conn) UpdateTime() {
	conn.LastUpdate = time.Now()
}

func (conn *Conn) keepAlive() {
	for {
		if conn.State == CLOSED || conn.State == CLOSING {
			return

		} else if conn.State == CONNECTED {
			tcpPacket := header.BuildTcpPacket(conn.localAddress, uint16(conn.localPort), conn.remoteAddress,
				uint16(conn.remotePort), uint32(conn.nextSEQ), uint32(conn.nextACK), header.ACK, []byte{})
			conn.WriteWithHeader(tcpPacket)
		}
		time.Sleep(time.Second)
	}
}

//Block needs upto 40 bytes extra :(
func (conn *Conn) Read(b []byte) (n int, err error) {
	n, err = conn.ReadWithHeader(b)
	if err != nil {
		return 0, err
	}
	var hdr header.TCP
	hdr.Unmarshal(b)
	copy(b, b[hdr.HeaderLen():])
	return n - int(hdr.HeaderLen()), err
}

//Block
func (conn *Conn) Write(b []byte) (n int, err error) {

	return conn.WriteWithHeader(header.BuildTcpPacket(conn.localAddress, uint16(conn.localPort), conn.remoteAddress,
		uint16(conn.remotePort), uint32(conn.nextSEQ), uint32(conn.nextACK), header.ACK, b))
}

//Blocks and Reads
func (conn *Conn) ReadWithHeader(b []byte) (n int, err error) {

	n, from, err := conn.internalConn.ReadFromIP(b)
	if err != nil {
		return n, err
	}
	ip, _ := netinfo.B2ip(from.IP)
	var hdr header.TCP
	hdr.Unmarshal(b)
	for err == nil && (ip != conn.remoteAddress || hdr.DstPort != uint16(conn.localPort)) {
		// Also verify port and checksum !!!
		// Maybe can ignore checksum
		n, from, err = conn.internalConn.ReadFromIP(b)
		if err != nil {
			return n, err
		}
	
		ip, _ = netinfo.B2ip(from.IP)
		hdr.Unmarshal(b)
		// fmt.Print(hdr)
	}
	if err != nil {
		return n, err
	}
	hdr_len := int(hdr.HeaderLen())
	fmt.Println("\n", hdr.Seq, n, n-hdr_len, conn.nextACK)
	conn.nextACK = int(hdr.Seq) + n - hdr_len
	if hdr.Flags&header.SYN > 0 {
		conn.nextACK += 1
	}
	return n, err
}

//B
func (conn *Conn) WriteWithHeader(b []byte) (n int, err error) {

	conn.nextSEQ += len(b) - 20
	// fmt.Print("Writing\n")
	return conn.internalConn.Write(b)
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
		uint16(conn.remotePort), uint32(conn.nextSEQ), uint32(conn.nextACK), header.FIN|header.ACK, []byte{})

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
			// fmt.Print(hdr.Flags)
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
	conn.nextSEQ++
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
	return conn.internalConn.SetDeadline(t)
}

func (conn *Conn) SetReadDeadline(t time.Time) error {
	return nil
}

func (conn *Conn) SetWriteDeadline(t time.Time) error {
	return nil
}

// TODO: implement
func (conn *Conn) LocalAddr() net.Addr {
	return &net.UDPAddr{}
}

func (conn *Conn) RemoteAddr() net.Addr {
	return &net.UDPAddr{}
}