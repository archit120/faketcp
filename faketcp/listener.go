package faketcp

import (
	"net"
	"strconv"
	"strings"
)

var LISTENERBUFSIZE = 1024

func ListenPacket(proto, addr string) (*PacketConn, error) {
	tcpSqautter, err := net.Listen("tcp", addr)
	if err != nil {
		return nil, err
	}

	internalConn, err := net.ListenIP("ip4:6", nil)
	if err != nil {
		return nil, err
	}

	ip := strings.Split(addr, ":")
	localPort, err := strconv.Atoi(ip[1])
	if err != nil {
		return nil, err
	}

	tcpSqautter.Close()
	return NewPacketConn(0, localPort, internalConn, tcpSqautter.(*net.TCPListener)), nil
}

// type Listener struct {
// 	Address    string
// 	InputChan  chan string
// 	OutputChan chan string

// 	connectedCache *cache.Cache
// }

// func NewListener(addr string) (*Listener, error) {
// 	listener := &Listener{
// 		Address:    addr,
// 		InputChan:  make(chan string, LISTENERBUFSIZE),
// 		OutputChan: make(chan string, LISTENERBUFSIZE),

// 		connectedCache: cache.New(15*time.Second, 1*time.Minute),
// 	}
// 	listener.sendResponse()
// 	return listener, nil
// }

// func (l *Listener) sendResponse() {
// 	go func() {
// 		for {
// 			items := l.requestCache.Items()
// 			for src := range items {
// 				if respi, ok := l.requestCache.Get(src); ok {
// 					resp := respi.(string)
// 					l.OutputChan <- resp
// 				}
// 			}
// 			time.Sleep(time.Millisecond * 500)
// 		}
// 	}()
// }

// func (l *Listener) Accept() (net.Conn, error) {
// 	for {
// 		packet := <-l.InputChan
// 		_, ipHeader, _, tcpHeader, data, _ := header.Get([]byte(packet))
// 		src, dst := header.GetTcpAddr(ipHeader, tcpHeader)
// 		if tcpHeader.Flags == header.SYN && len(data) == 0 {
// 			seq, ack := 0, tcpHeader.Seq+1
// 			ipHeaderTo, tcpHeaderTo := header.BuildTcpHeader(dst, src)
// 			tcpHeaderTo.Seq, tcpHeaderTo.Ack = uint32(seq), uint32(ack)
// 			tcpHeaderTo.Flags = (header.SYN | header.ACK)
// 			response := string(header.BuildTcpPacket(ipHeaderTo, tcpHeaderTo, []byte{}))
// 			l.requestCache.Set(src, response, cache.DefaultExpiration)
// 			l.OutputChan <- response

// 		} else if tcpHeader.Flags == header.ACK {
// 			if _, ok := l.requestCache.Get(src); ok {
// 				l.requestCache.Delete(src)
// 				conn := NewConn(dst, src, CONNECTED)
// 				faketcpServer.CreateConn(dst, src, conn)
// 				return conn, nil
// 			}
// 		}
// 	}
// }

// func (l *Listener) Close() error {
// 	go func() {
// 		defer func() {
// 			recover()
// 		}()
// 		close(l.InputChan)
// 	}()

// 	go func() {
// 		defer func() {
// 			recover()
// 		}()
// 		close(l.OutputChan)
// 	}()
// 	faketcpServer.CloseListener(l.Address)
// 	return nil
// }

// func (l *Listener) Addr() net.Addr {
// 	return nil
// }
