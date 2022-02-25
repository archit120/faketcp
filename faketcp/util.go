package faketcp

import (
	"net"
	"time"

	"github.com/patrickmn/go-cache"
)
var localAddressCache cache.Cache = *cache.New(time.Minute*5, time.Minute*10)

func GetLocalAddr(remoteAddr string) (net.Addr, error) {
	
	conn, err := net.Dial("udp", remoteAddr)
	if err != nil {
		return nil, err
	}
	return conn.LocalAddr(), nil
}