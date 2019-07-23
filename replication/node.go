package replication

import (
	"fmt"
	"log"
	"math/big"
	"net"

	"github.com/hashicorp/memberlist"
	"github.com/labstack/echo"

	memberlistInterface "github.com/alexandrestein/gotinydb/replication/memberlist"
	"github.com/alexandrestein/securelink"
)

type (
	Node struct {
		Echo        *echo.Echo
		Certificate *securelink.Certificate

		Server     *securelink.Server
		Memberlist *memberlist.Memberlist
	}
)

func NewNode(port uint16, cert *securelink.Certificate) (_ *Node, err error) {
	n := new(Node)

	n.Certificate = cert

	err = n.startServer(port)
	if err != nil {
		return nil, err
	}

	err = n.startMemberlist()
	if err != nil {
		return nil, err
	}

	return n, nil
}

func (n *Node) startServer(port uint16) error {
	tlsConfig := securelink.GetBaseTLSConfig(n.GetID().String(), n.Certificate)

	// tlsListener, err := tls.Listen("tcp", fmt.Sprintf(":%s", n.GetPort()), tlsConfig)
	// if err != nil {
	// 	return err
	// }

	s, err := securelink.NewServer(port, tlsConfig, n.Certificate, n.GetIDFromAddr)
	if err != nil {
		return err
	}
	n.Server = s

	// cl := securelink.NewListener(tlsListener)
	// cl.RegisterService("raft", , n.raftTransport)

	// n.Echo = echo.New()
	// n.Echo.Server = &http.Server{
	// 	TLSConfig: tlsConfig,
	// }
	// n.Echo.TLSListener = cl

	// n.settupHandlers()

	// httpServer := &http.Server{
	// 	TLSConfig: tlsConfig,
	// 	Handler:   nil,
	// }

	go func() {
		// err := n.Echo.StartServer(httpServer)
		err = s.Start()
		fmt.Println("merde avec le sever", err)
		log.Fatal(err)
	}()

	return nil
}

func (n *Node) startMemberlist() error {
	isMemberlist := func(serverName string) bool {
		if CheckMemberlistHostRequestReg.MatchString(serverName) {
			return true
		}

		return false
	}

	mt := memberlistInterface.NewMemberlistTransport(n.Server)

	handler := securelink.NewHandler("memberlist", isMemberlist, mt.Handle)
	n.Server.RegisterService(handler)

	// mConfig := memberlist.DefaultWANConfig()
	mConfig := memberlist.DefaultLocalConfig()
	mConfig.DisableTcpPings = true
	mConfig.AdvertisePort = int(n.Server.AddrStruct.Port)
	mConfig.Transport = memberlistInterface.NewMemberlistTransport(n.Server)

	list, err := memberlist.Create(mConfig)
	if err != nil {
		return err
	}

	n.Memberlist = list

	return nil
}

func (n *Node) GetToken() (string, error) {
	_, port, err := net.SplitHostPort(n.Server.AddrStruct.String())
	if err != nil {
		return "", err
	}
	return n.Certificate.GetToken(port)
}

func (n *Node) GetID() *big.Int {
	return n.Certificate.Cert.SerialNumber
}

// func (n *Node) buildRaftTransport() {
// 	n.raftTransport = &Transport{
// 		acceptChan:    make(chan *transportConn),
// 		addr:          n.Addr,
// 		cert:          n.Certificate,
// 		getIDFromAddr: n.getIDFromAddr,
// 	}
// }

func (n *Node) GetIDFromAddr(addr string) (serverID string) {
	// for i, server := range n.Raft.GetConfiguration().Configuration().Servers {
	// 	fmt.Println("GetIDFromAddr server i", i, server)
	// 	if server.Address == raft.ServerAddress(addr) {
	// 		return string(server.ID)
	// 	}
	// }
	return n.getIDFromAddrByConnecting(addr)
}

// func (n *Node) getIDFromAddrByConnecting(addr string) (serverID string) {
// 	tlsConfig := securelink.GetBaseTLSConfig("", n.Certificate)
// 	tlsConfig.InsecureSkipVerify = true
// 	conn, err := tls.Dial("tcp", string(addr), tlsConfig)
// 	if err != nil {
// 		fmt.Println("err -1", err)
// 		return ""
// 	}

// 	err = conn.Handshake()
// 	if err != nil {
// 		fmt.Println("err 0", err)
// 		return ""
// 	}

// 	remoteCert := conn.ConnectionState().PeerCertificates[0]
// 	err = remoteCert.CheckSignatureFrom(n.Certificate.CACert)
// 	if err != nil {
// 		fmt.Println("err 1", err)
// 		return ""
// 	}

// 	return remoteCert.SerialNumber.String()
// }

func (n *Node) Join(addrs []string) (int, error) {
	return n.Memberlist.Join(addrs)
}

// func (n *Node) getHostAndPort() (string, string) {
// 	host, port, err := net.SplitHostPort(n.Addr.String())
// 	if err != nil {
// 		return "", ""
// 	}
// 	return host, port
// }
