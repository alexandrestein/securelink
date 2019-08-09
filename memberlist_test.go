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
	defer s0.Close()
	mbConf := memberlist.DefaultLocalConfig()
	mbConf.Name = "0"
	err := s0.StartMemberlist(mbConf)
	if err != nil {
		t.Error(err)
		return
	}

	nullWriter, _ := os.OpenFile(os.DevNull, os.O_WRONLY, 0666)

	s1 := servers[1]
	defer s1.Close()
	mbConf = memberlist.DefaultLocalConfig()
	mbConf.Name = "1"
	mbConf.LogOutput = nullWriter
	err = s1.StartMemberlist(mbConf)
	if err != nil {
		t.Error(err)
		return
	}

	s2 := servers[2]
	defer s2.Close()
	mbConf = memberlist.DefaultLocalConfig()
	mbConf.Name = "2"
	mbConf.LogOutput = nullWriter
	err = s2.StartMemberlist(mbConf)
	if err != nil {
		t.Error(err)
		return
	}

	time.Sleep(time.Millisecond * 500)


	_, err = s0.Memberlist.Join([]string{s1.AddrStruct.String(),	s2.AddrStruct.String()})
	// _, err = s0.Memberlist.Join([]string{s1.AddrStruct.String()})
	if err != nil {
		t.Error(err)
		return
	}

	// time.Sleep(time.Second * 5)
	// _, err = s0.Memberlist.Join([]string{s2.AddrStruct.String()})
	// if err != nil {
	// 	t.Error(err)
	// 	return
	// }

	time.Sleep(time.Second * 6)

	if score := s0.Memberlist.GetHealthScore(); score != 0 {
		t.Errorf("the expected health score is 0 but has %d", score)
	}

	if nb := s0.Memberlist.NumMembers(); nb != 3 {
		t.Errorf("the expected numbers of node is 3 but has %d", nb)
		t.Log(s0.Memberlist.Members(), s1.Memberlist.Members(), s2.Memberlist.Members())
	}
}
