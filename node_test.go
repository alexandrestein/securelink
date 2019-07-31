package securelink_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/alexandrestein/securelink"
)

func TestNode(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	_, servers := initServers(ctx, 3)

	s0 := servers[0]
	defer s0.Close()

	s1 := servers[1]
	defer s1.Close()

	s2 := servers[2]
	defer s2.Close()

	config := &securelink.Peer{}

	config.Priority = 0.9
	n0, _ := securelink.NewNode(s0, config)
	config.Priority = 0.5
	n1, _ := securelink.NewNode(s1, config)
	config.Priority = 0.2
	n2, _ := securelink.NewNode(s2, config)

	fmt.Println("master is " + n0.Server.Certificate.ID().String())

	time.Sleep(time.Second)

	err := n0.AddPeer(n1.LocalConfig)
	if err != nil {
		t.Error(err)
		return
	}

	config.Priority = 0.5
	err = n0.AddPeer(config)
	if err == nil {
		t.Errorf("expected an error but had no")
		return
	}

	time.Sleep(time.Second * 5)
	err = n0.AddPeer(n2.LocalConfig)
	if err != nil {
		t.Error(err)
		return
	}
	time.Sleep(time.Second * 30)
}
