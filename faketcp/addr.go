package faketcp

type Addr struct {
	addr [4]byte
}

func NewAddr(addr string) *Addr {
	return &Addr{
		addr: addr,
	}
}

func (a *Addr) Network() string {
	return "faketcp"
}

func (a *Addr) String() string {
	return a.addr
}
