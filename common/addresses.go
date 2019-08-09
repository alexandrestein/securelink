package common

import (
	"fmt"
	"net"
	"sync"
)

type (
	Addr struct {
		lock sync.RWMutex

		mainAddr string
		port     uint16
		addrs    []string
	}
)

func NewAddr(port uint16) (*Addr, error) {
	addrs, err := GetAddresses()
	if err != nil {
		return nil, err
	}

	if len(addrs) == 0 {
		return nil, fmt.Errorf("no address found")
	}

	return &Addr{
		lock:     sync.RWMutex{},
		mainAddr: addrs[0],
		port:     port,
		addrs:    addrs,
	}, nil
}

func (a *Addr) SwitchMain(i int) string {
	a.lock.Lock()
	defer a.lock.Unlock()

	if i > len(a.addrs)-1 {
		return ""
	}
	a.mainAddr = a.addrs[i]
	return a.String()
}

func (a *Addr) String() string {
	a.lock.RLock()
	defer a.lock.RUnlock()

	return fmt.Sprintf("%s:%d", a.mainAddr, a.port)
}

func (a *Addr) MainAddr() string {
	a.lock.RLock()
	defer a.lock.RUnlock()

	return a.mainAddr
}

func (a *Addr) Network() string {
	return "udp"
}

func (a *Addr) Port() int {
	a.lock.RLock()
	defer a.lock.RUnlock()

	return int(a.port)
}

func (a *Addr) Addrs() []string {
	a.lock.RLock()
	defer a.lock.RUnlock()

	return a.addrs
}

func (a *Addr) ForListenerBroadcast() string {
	a.lock.RLock()
	defer a.lock.RUnlock()

	return fmt.Sprintf(":%d", a.Port)
}

func (a *Addr) IP() net.IP {
	a.lock.RLock()
	defer a.lock.RUnlock()

	return net.ParseIP(a.mainAddr)
}

func (a *Addr) IPsV4() (ret []string) {
	a.lock.RLock()
	defer a.lock.RUnlock()

	ret = []string{}
	for _, ipStr := range a.addrs {
		ip := net.ParseIP(ipStr)
		if ip.To4() != nil {
			ret = append(ret, ipStr)
		}
	}
	return
}

func (a *Addr) IPsV6() (ret []string) {
	a.lock.RLock()
	defer a.lock.RUnlock()

	ret = []string{}
	for _, ipStr := range a.addrs {
		ip := net.ParseIP(ipStr)
		if ip.To16() != nil {
			ret = append(ret, ipStr)
		}
	}
	return
}

func (a *Addr) UDPAddr() (*net.UDPAddr, error) {
	return net.ResolveUDPAddr("udp", a.String())
}

func (a *Addr) MustUDPAddr() *net.UDPAddr {
	udpAddr, _ := a.UDPAddr()
	return udpAddr
}

func GetAddresses() ([]string, error) {
	interfaces, err := net.Interfaces()
	if err != nil {
		return nil, err
	}

	ret := []string{}

	for _, nic := range interfaces {
		var addrs []net.Addr
		addrs, err = nic.Addrs()
		if err != nil {
			return nil, err
		}

		for _, addr := range addrs {
			ipAsString := addr.String()
			ip, _, err := net.ParseCIDR(ipAsString)
			if err != nil {
				continue
			}

			ipAsString = ip.String()
			ip2 := net.ParseIP(ipAsString)
			if to4 := ip2.To4(); to4 == nil {
				ipAsString = ipAsString
			}

			// If ip accessible from outside
			if ip.IsGlobalUnicast() {
				ret = append(ret, ipAsString)
			}
		}
	}

	return ret, nil
}
