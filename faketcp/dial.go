package faketcp

import (
	"encoding/binary"
	"fmt"
	"math/rand"
	"net"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/archit120/faketcp/header"
	"github.com/archit120/faketcp/netinfo"
)

const (
	RETRYTIME     = 5
	RETRYINTERVAL = 500
	BUFFERSIZE = 65535
)

func Dial(proto string, remoteAddr string) (net.Conn, error) {
	localAddr, err := GetLocalAddr(remoteAddr)
	if err != nil {
		return nil, err
	}
	fd, err := syscall.Socket(syscall.AF_INET, syscall.SOCK_RAW, syscall.IPPROTO_TCP)
	if err != nil {
		return nil, err
	}

	ip := strings.Split(remoteAddr, ":")
	ipu, err := netinfo.S2ip(ip[0])
	if err != nil {
		return nil, err
	}
	ipb := make([]byte, 4)
	binary.BigEndian.PutUint32(ipb, ipu)

	ips, err := netinfo.GetSrcIpForDst(ipu)
	if err != nil {
		return nil, err
	}
	ipbs := make([]byte, 4)
	binary.BigEndian.PutUint32(ipbs, ips)

	localPort := uint16(rand.Int())
	remotePort, err := strconv.Atoi(ip[1])
	conn := NewConn(ipbs, int(localPort), ipb, remotePort, CONNECTING, fd)
	tcpHeader := header.BuildTcpHeader(localAddr.String(), remoteAddr)
	tcpHeader.Seq = 0
	tcpHeader.Flags = header.SYN
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
			if tcpHeader.Flags == (header.SYN|header.ACK) && tcpHeader.Ack == 1 {
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
		return nil, err
	}

	//seq, ack := 1, tcpHeader.Seq+1
	ipHeader, tcpHeader = header.BuildTcpHeader(localAddr.String(), remoteAddr)
	tcpHeader.Seq = 1
	tcpHeader.Ack = 1
	tcpHeader.Flags = header.ACK
	packet = header.BuildTcpPacket(ipHeader, tcpHeader, []byte{})

	n, err := conn.WriteWithHeader(packet)
	if err != nil || n != len(packet) {
		return nil, fmt.Errorf("packet loss (expect=%v, real=%v) or %v", len(packet), n, err)
	}
	conn.State = CONNECTED
	return conn, nil
}
