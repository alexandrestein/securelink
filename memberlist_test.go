package securelink_test

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/hashicorp/memberlist"
)

func TestMemberlist(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*300)
	defer cancel()
	_, servers := initServers(ctx, 3, 0)

	s0 := servers[0]
	// s0.AddrStruct.Addrs = s0.AddrStruct.IPsV4()
	// s0.AddrStruct.SwitchMain(3)
	mbConf := memberlist.DefaultLocalConfig()
	mbConf.Name = "0"
	// mbConf.DisableTcpPings = true
	// mbConf.EnableCompression = true
	err := s0.StartMemberlist(mbConf)
	if err != nil {
		t.Error(err)
		return
	}

	nullWriter, _ := os.OpenFile(os.DevNull, os.O_WRONLY, 0666)

	s1 := servers[1]
	// s1.AddrStruct.Addrs = s1.AddrStruct.IPsV4()
	// s1.AddrStruct.SwitchMain(1)
	mbConf = memberlist.DefaultLocalConfig()
	mbConf.Name = "1"
	// mbConf.DisableTcpPings = true
	// mbConf.EnableCompression = true
	mbConf.LogOutput = nullWriter
	err = s1.StartMemberlist(mbConf)
	if err != nil {
		t.Error(err)
		return
	}

	s2 := servers[2]
	// s2.AddrStruct.Addrs = s2.AddrStruct.IPsV4()
	// s2.AddrStruct.SwitchMain(2)
	mbConf = memberlist.DefaultLocalConfig()
	mbConf.Name = "2"
	// mbConf.DisableTcpPings = true
	// mbConf.EnableCompression = true
	mbConf.LogOutput = nullWriter
	err = s2.StartMemberlist(mbConf)
	if err != nil {
		t.Error(err)
		return
	}

	time.Sleep(time.Second * 2)

	// _, err = s0.Memberlist.Join([]string{fmt.Sprintf("127.0.0.1:%d", s1.AddrStruct.Port+10), fmt.Sprintf("127.0.0.1:%d", s2.AddrStruct.Port+10)})

	_, err = s0.Memberlist.Join([]string{s1.AddrStruct.String(), s2.AddrStruct.String()})
	if err != nil {
		t.Error(err)
		return
	}

	time.Sleep(time.Second * 15)

	// _, err = s2.Memberlist.Join([]string{fmt.Sprintf("127.0.0.1:%d", s1.AddrStruct.Port+10)})
	// _, err = s0.Memberlist.Join([]string{s2.AddrStruct.String()})
	// if err != nil {
	// 	t.Error(err)
	// 	return
	// }

	time.Sleep(time.Second * 5)

	if score := s0.Memberlist.GetHealthScore(); score != 0 {
		t.Errorf("the expected health score is 0 but has %d", score)
	}

	if nb := s0.Memberlist.NumMembers(); nb != 3 {
		t.Errorf("the expected numbers of node is 3 but has %d", nb)
		t.Log(s0.Memberlist.Members(), s1.Memberlist.Members(), s2.Memberlist.Members())
	}

}
