package raw

import (
	"fmt"
	"os"
	"syscall"

	"github.com/xitongsys/ptcp/header"
	"github.com/mdlayher/ethernet"
)

var RAWBUFSIZE = 65535

type Raw struct {
	fdWrite int
	fdRead int
	readFile *os.File
	buf []byte
}

func NewRaw() (*Raw, error){
	fdW, err := syscall.Socket(syscall.AF_INET, syscall.SOCK_RAW, syscall.IPPROTO_RAW)
	if err != nil {
		return nil, err
	}

	fdR, err := syscall.Socket(syscall.AF_PACKET, syscall.SOCK_RAW, syscall.ETH_P_ALL)
	if err != nil {
		return nil, err
	}
	readF := os.NewFile(uintptr(fdR), fmt.Sprintf("fd %d", fdR))

	return &Raw{
		fdWrite: fdW, 
		fdRead: fdR,
		readFile: readF,
		buf: make([]byte, RAWBUFSIZE),
	}, nil
}

func (r *Raw) Read() ([]byte, error) {
	n, err := r.readFile.Read(r.buf)
	if err == nil {
		eth := &ethernet.Frame{}
		eth.UnmarshalBinary(r.buf[:n])
		return eth.Payload, err
	}
	return nil, err
}

func (r *Raw) Write(data []byte, addrs string) error {
	ip, port := header.ParseAddr(addrs)

	addr := syscall.SockaddrInet4 {
		Port: port,
		Addr: header.IpStr2Bytes(ip),
	}
	return syscall.Sendto(r.fdWrite, data, 0, &addr)
}