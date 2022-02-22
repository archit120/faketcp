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
	if err != nil {
		return nil, err
	}

	conn := NewConn(ips, int(localPort), ipu, remotePort, CONNECTING, fd)
	tcpPacket := header.BuildTcpPacket(conn.localAddress, uint16(conn.localPort), conn.remoteAddress,
	uint16(conn.remotePort), uint32(conn.nextSEQ), uint32(conn.nextACK), header.SYN, []byte{})

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
			if hdr.Flags == (header.ACK | header.SYN) {
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
	conn.nextSEQ++

	//seq, ack := 1, tcpHeader.Seq+1
	conn.State = CONNECTED
	return conn, nil
}
