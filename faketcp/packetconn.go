package faketcp

import (
	"fmt"
	"net"
	"strconv"
	"time"

	"github.com/archit120/faketcp/header"
	"github.com/archit120/faketcp/netinfo"
	"github.com/patrickmn/go-cache"
)

var PACKETCONNBUFFERSIZE = 1024

type IpPortPair struct {
	Addr *net.IPAddr
	Port int
}

type dataAdressPair struct {
	address *IpPortPair
	data    []byte
}

type PacketConn struct {
	localPort  int
	internalConn *net.IPConn
	// fd         int
	InputChan  chan dataAdressPair
	OutputChan chan dataAdressPair
	nextAck    *cache.Cache
	nextSEQ    *cache.Cache
}

func NewPacketConn(localAddr uint32, localPort int, internalConn *net.IPConn) *PacketConn {
	conn := &PacketConn{
		localPort:  localPort,
		internalConn:         internalConn,
		nextAck:    cache.New(15*time.Second, 1*time.Minute),
		nextSEQ:    cache.New(15*time.Second, 1*time.Minute),
		InputChan:  make(chan dataAdressPair, PACKETCONNBUFFERSIZE),
		OutputChan: make(chan dataAdressPair, PACKETCONNBUFFERSIZE),
	}

	go conn.bgReader()
	// go conn.bgWriter()

	return conn
}

func (conn *PacketConn) bgReader() {
	for {
		b := make([]byte, 1500)
		n, from, err := conn.internalConn.ReadFromIP(b)

		var hdr header.TCP
		hdr.Unmarshal(b)
		for err == nil && hdr.DstPort != uint16(conn.localPort) {
			// Maybe can ignore checksum
			n, from, err = conn.internalConn.ReadFromIP(b)
			hdr.Unmarshal(b)
		}

		if err != nil {
			fmt.Errorf("Error occured in PacketConn bgReader")
			fmt.Print(err)
			return
		}
		fromPair := &IpPortPair{
			Addr: from,
			Port: int(hdr.SrcPort),
		}
		if hdr.Flags&header.SYN > 0 {
			conn.acceptConnection(hdr, fromPair)
		} else if hdr.Flags&header.FIN > 0 {
			conn.closeConnection(hdr, fromPair)
		} else {
			n = copy(b, b[hdr.HeaderLen():n])
			remoteAdrr, _ := netinfo.B2ip(from.IP)
			key := getKey(int(hdr.SrcPort), remoteAdrr)
			var nextACK = hdr.Seq + uint32(n)
			conn.nextAck.Set(key, nextACK, cache.DefaultExpiration)

			conn.InputChan <- dataAdressPair{
				address: fromPair,
				data:    b[:n],
			}
		}
	}
}

func getKey(port int, remoteAddr uint32) string {
	return netinfo.Ip2s(remoteAddr) + ":" + strconv.Itoa(port)
}

func (conn *PacketConn) acceptConnection(tcpHeader header.TCP, from *IpPortPair) {
	remoteAdrr, _ := netinfo.B2ip(from.Addr.IP)
	localAddr, err := netinfo.B2ip(conn.internalConn.LocalAddr().(*net.IPAddr).IP)
	if err != nil {
		fmt.Errorf("Error in acceptConnection while finding local address to use\n")
		return
	}
	tcpPacket := header.BuildTcpPacket(localAddr, uint16(conn.localPort), remoteAdrr,
		uint16(from.Port), uint32(1), uint32(tcpHeader.Seq+1), header.SYN|header.ACK, []byte{})

	conn.nextAck.Set(getKey(from.Port, remoteAdrr), uint32(tcpHeader.Seq+1), cache.DefaultExpiration)
	conn.nextSEQ.Set(getKey(from.Port, remoteAdrr), uint32(2), cache.DefaultExpiration)

	conn.internalConn.WriteToIP(tcpPacket, from.Addr)
}

func (conn *PacketConn) closeConnection(tcpHeader header.TCP, from *IpPortPair) {
	remoteAdrr, _ := netinfo.B2ip(from.Addr.IP)
	localAddr, err := netinfo.GetSrcIpForDst(remoteAdrr)
	if err != nil {
		fmt.Errorf("Error in closeConnection while finding local address to use\n")
		return
	}
	key := getKey(from.Port, remoteAdrr)
	nextSeq, d := conn.nextSEQ.Get(key)
	if !d {
		nextSeq = uint32(1)
	}
	nextAck, d := conn.nextAck.Get(key)
	if !d {
		nextAck = uint32(1)
	}
	tcpPacket := header.BuildTcpPacket(localAddr, uint16(conn.localPort), remoteAdrr,
		uint16(from.Port), nextSeq.(uint32), nextAck.(uint32), header.FIN|header.ACK, []byte{})

	conn.nextAck.Delete(key)
	conn.internalConn.WriteToIP(tcpPacket, from.Addr)
}

// func (conn *PacketConn) bgWriter() {

// }

// //Block needs upto 40 bytes extra :(
func (conn *PacketConn) ReadFrom(b []byte) (n int, addr *IpPortPair, err error) {
	d := <-conn.InputChan
	return copy(b, d.data), d.address, nil
}

func (conn *PacketConn) WriteTo(p []byte, addr *IpPortPair) (n int, err error) {
	remoteAdrr, _ := netinfo.B2ip(addr.Addr.IP)
	localAddr, err := netinfo.GetSrcIpForDst(remoteAdrr)
	key := getKey(addr.Port, remoteAdrr)
	nextSeq, d := conn.nextSEQ.Get(key)
	if !d {
		nextSeq = uint32(1)
	}
	nextAck, d := conn.nextAck.Get(key)
	if !d {
		nextAck = uint32(1)
	}
	packet := header.BuildTcpPacket(localAddr, uint16(conn.localPort), remoteAdrr, uint16(addr.Port), nextSeq.(uint32), nextAck.(uint32), header.ACK, p)
	conn.nextSEQ.Set(key, uint32(nextSeq.(uint32)+uint32(len(p))), cache.DefaultExpiration)
	return conn.internalConn.WriteToIP(packet, addr.Addr)
}

// WriteTo(p []byte, addr Addr) (n int, err error)

// //Block
// func (conn *PacketConn) Write(b []byte) (n int, err error) {

// 	return conn.WriteWithHeader(header.BuildTcpPacket(conn.localAddress, uint16(conn.localPort), conn.remoteAddress,
// 		uint16(conn.remotePort), uint32(conn.nextSEQ), uint32(conn.nextACK), header.SYN|header.ACK, b))
// }

// //Blocks and Reads
// func (conn *PacketConn) ReadWithHeader(b []byte) (n int, err error) {

// 	n, from, err := syscall.Recvfrom(conn.fd, b, 0)
// 	ip, _ := netinfo.B2ip(from.(*syscall.SockaddrInet4).Addr[:])
// 	var hdr header.TCP
// 	hdr.Unmarshal(b[20:])
// 	for err == nil && hdr.DstPort != uint16(conn.localPort) {
// 		// Also verify port and checksum !!!
// 		// Maybe can ignore checksum
// 		n, from, err = syscall.Recvfrom(conn.fd, b, 0)
// 		ip, _ = netinfo.B2ip(from.(*syscall.SockaddrInet4).Addr[:])
// 		hdr.Unmarshal(b[20:])
// 	}
// 	if err != nil {
// 		return 0, err
// 	}
// 	n = copy(b, b[20:n])
// 	conn.nextACK = int(hdr.Seq) + n + int(hdr.Flags&header.SYN)
// 	return n, err
// }

// //B
// func (conn *PacketConn) WriteWithHeader(b []byte) (n int, err error) {
// 	var remoteadd [4]byte
// 	binary.BigEndian.PutUint32(remoteadd[:], conn.remoteAddress)
// 	to := syscall.SockaddrInet4{
// 		Port: conn.remotePort,
// 		Addr: remoteadd,
// 	}
// 	conn.nextSEQ += len(b) - 20
// 	return len(b), syscall.Sendto(conn.fd, b, 0, &to)
// }

// func (conn *PacketConn) CloseRequest() (err error) {
// 	if conn.State != CONNECTED {
// 		return nil
// 	}

// 	defer func() {
// 		conn.State = CLOSED
// 	}()

// 	conn.State = CLOSING
// 	tcpPacket := header.BuildTcpPacket(conn.localAddress, uint16(conn.localPort), conn.remoteAddress,
// 		uint16(conn.remotePort), uint32(conn.nextSEQ), uint32(conn.nextACK), header.FIN, []byte{})

// 	done := make(chan int)
// 	go func() {
// 		for i := 0; i < RETRYTIME; i++ {
// 			select {
// 			case <-done:
// 				return
// 			default:
// 			}
// 			conn.WriteWithHeader(tcpPacket)
// 			time.Sleep(time.Millisecond * RETRYINTERVAL)
// 		}
// 	}()

// 	after := time.After(time.Millisecond * RETRYINTERVAL * RETRYTIME)
// 	buf := make([]byte, BUFFERSIZE)
// 	timeOut := false
// 	for !timeOut {
// 		if n, err := conn.ReadWithHeader(buf); n > 0 && err == nil {
// 			var hdr header.TCP
// 			hdr.Unmarshal(buf)
// 			if hdr.Flags == (header.ACK | header.FIN) {
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

// 	if err != nil {
// 		return err
// 	}

// 	tcpPacket = header.BuildTcpPacket(conn.localAddress, uint16(conn.localPort), conn.remoteAddress,
// 		uint16(conn.remotePort), uint32(conn.nextSEQ), uint32(conn.nextACK), header.ACK, []byte{})
// 	conn.WriteWithHeader(tcpPacket)

// 	return nil
// }

// func (conn *PacketConn) CloseResponse() (err error) {
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

func (conn *PacketConn) Close() error {
	// conn.CloseRequest()
	return conn.internalConn.Close()
}
func (conn *PacketConn) SetDeadline(t time.Time) error {
	return nil
}

func (conn *PacketConn) SetReadDeadline(t time.Time) error {
	return nil
}

func (conn *PacketConn) SetWriteDeadline(t time.Time) error {
	return nil
}

// TODO: implement
func (conn *PacketConn) LocalAddr() net.Addr {
	return &net.UDPAddr{}
}

func (conn *PacketConn) RemoteAddr() net.Addr {
	return &net.UDPAddr{}
}
