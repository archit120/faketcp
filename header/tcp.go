// Taken from https://github.com/xitongsys/ethernet-go/blob/master/header/tcp.go

package header

import (
	"encoding/binary"
	"fmt"
)

type TCP struct {
	SrcPort    uint16
	DstPort    uint16
	Seq        uint32
	Ack        uint32
	Offset     uint8
	Flags      uint8
	Win        uint16
	Checksum   uint16
	UrgPointer uint16
	Opt        uint32
}

const (
	FIN = 0x01
	SYN = 0x02
	RST = 0x04
	PSH = 0x08
	ACK = 0x10
	URG = 0x20
	ECE = 0x40
	CWR = 0x80
)


func (h TCP) String() string {
	res := `
{
	SrcPort: %d,
	DstPort: %d,
	Seq: %d,
	Ack: %d,
	Offset: %d,
	Flags: %d,
	Win: %d,
	Checksum: %d,
	UrgPointer: %d,
}
`
	return fmt.Sprintf(res, h.SrcPort, h.DstPort,
		h.Seq, h.Ack,
		h.Offset, h.Flags, h.Win,
		h.Checksum, h.UrgPointer)
}

func (h *TCP) HeaderLen() uint16 {
	return (uint16(h.Offset) >> 4) * 4
}

func (h *TCP) Marshal() []byte {
	res := make([]byte, 20)
	binary.BigEndian.PutUint16(res, h.SrcPort)
	binary.BigEndian.PutUint16(res[2:], h.DstPort)
	binary.BigEndian.PutUint32(res[4:], h.Seq)
	binary.BigEndian.PutUint32(res[8:], h.Ack)
	res[12] = byte(h.Offset)
	res[13] = byte(h.Flags)
	binary.BigEndian.PutUint16(res[14:], h.Win)
	binary.BigEndian.PutUint16(res[16:], h.Checksum)
	binary.BigEndian.PutUint16(res[18:], h.UrgPointer)
	return res
}

func (h *TCP) Unmarshal(bs []byte) error {
	if len(bs) < 20 {
		return fmt.Errorf("too short")
	}
	h.SrcPort = binary.BigEndian.Uint16(bs[0:2])
	h.DstPort = binary.BigEndian.Uint16(bs[2:4])
	h.Seq = binary.BigEndian.Uint32(bs[4:8])
	h.Ack = binary.BigEndian.Uint32(bs[8:12])
	h.Offset = uint8(bs[12])
	h.Flags = uint8(bs[13])
	h.Win = binary.BigEndian.Uint16(bs[14:16])
	h.Checksum = binary.BigEndian.Uint16(bs[16:18])
	h.UrgPointer = binary.BigEndian.Uint16(bs[18:20])

	return nil
}

func ReCalTcpCheckSum(ipsrc uint32, ipdst uint32, bs []byte) error {
	if len(bs) < 20 {
		return fmt.Errorf("too short")
	}
	ipps := IPv4Pseudo{}
	ipps.Src = ipsrc
	ipps.Dst = ipdst
	ipps.Reserved = 0
	ipps.Protocol = 6
	ipps.Len = uint16(len(bs))

	ippsbs := ipps.Marshal()
	tcpbs := bs

	// Set checksum bytes to 0
	tcpbs[16] = 0
	tcpbs[17] = 0

	if len(tcpbs) % 2 == 1 {
		tcpbs = append(tcpbs, byte(0))
	}

	s := uint32(0)
	for i := 0; i<len(ippsbs); i+=2 {
		s +=  uint32(binary.BigEndian.Uint16(ippsbs[i : i+2]))
	}
	for i := 0; i<len(tcpbs); i+=2 {
		s +=  uint32(binary.BigEndian.Uint16(tcpbs[i : i+2]))
	}
	
	for (s>>16) > 0 {
		s = (s>>16) + (s&0xffff)
	}

	// Insert checksum
	binary.BigEndian.PutUint16(tcpbs[16:], ^uint16(s))
	return nil
}

func BuildTcpPacket(ipsrc uint32, srcPort uint16, ipdst uint32, dstPort uint16, seq uint32, ack uint32, flag uint8, data []byte)  []byte {
	tcpHeader := &TCP{
		SrcPort: uint16(srcPort),
		DstPort: uint16(dstPort),
		Seq: seq,
		Ack: ack,
		Offset: 0x50, //hack for correct marshalling
		Flags: flag,
		Win: ^uint16(0),
		Checksum: 0,
		UrgPointer: 0,
	}
	result := make([]byte, len(data)+20)
	copy(result, tcpHeader.Marshal())
	copy(result[20:], data)
	ReCalTcpCheckSum(ipsrc, ipdst, result)
	return result
}