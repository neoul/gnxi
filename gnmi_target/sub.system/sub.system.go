package main

import (
	"fmt"
	"log"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"github.com/neoul/libydb/go/ydb"
)

// IfStats - Interface statistics
type IfStats struct {
	Name           string
	Type           string
	Mtu            uint16
	Enabled        string
	InetAddr       string
	Netmask        string
	Inet6Addr      string
	Inet6Prefixlen int
	Broadcast      string
	Ether          string
	RxPacket       uint64
	RxBytes        uint64
	RxError        uint64
	RxDrop         uint64
	RxOverruns     uint64
	RxFrame        uint64
	TxPacket       uint64
	TxBytes        uint64
	TxError        uint64
	TxDrop         uint64
	TxOverruns     uint64
	TxCarrier      uint64
	TxCollisions   uint64
}

// IfInfo - for NIC statistic
type IfInfo struct {
	Ifstats map[string]*IfStats
}

// SyncUpdate - updates datablock
func (sinfo *IfInfo) SyncUpdate(keys []string, key string) []byte {
	// fmt.Println(keys, key)
	sname, klist, err := ydb.ExtractStrKeyNameAndValue(key)
	if err != nil || sname != "interface" {
		return nil
	}
	// fmt.Println(sname, klist)
	if kvalue, ok := klist["name"]; ok {
		ns, err := collectStats(kvalue)
		if err == nil && len(ns) > 0 {
			sinfo.Ifstats[kvalue] = ns[0]
			b := marshal(ns[0])
			// fmt.Println(string(b))
			return b
		}
	}
	return nil
}

func split(s string) []string {
	ss := strings.Split(s, " ")
	ns := make([]string, 0, len(ss))
	for _, e := range ss {
		trimeds := strings.Trim(e, " \n")
		if trimeds != "" {
			ns = append(ns, trimeds)
		}
	}
	return ns
}

func newIfStats(ifinfo string) *IfStats {
	if ifinfo == "" {
		return nil
	}
	ifs := &IfStats{}
	defer func() {
		if r := recover(); r != nil {
			ifs = nil
			// fmt.Println("Recovered", r)
		}
	}()
	ifinfo = strings.Trim(ifinfo, " ")
	found := strings.Index(ifinfo, ": ")
	ifs.Name = ifinfo[0:found]
	ifinfolist := strings.Split(ifinfo, "\n")
	for _, s := range ifinfolist[1:] {
		item := split(s)
		if len(item) == 0 {
			continue
		}
		switch item[0] {
		case "inet":
			ifs.InetAddr = item[1]
			ifs.Netmask = item[3]
		case "inet6":
			ifs.Inet6Addr = item[1]
			ifs.Inet6Prefixlen, _ = strconv.Atoi(item[3])
		case "ether":
			ifs.Ether = item[1]
		case "RX":
			if item[1] == "packets" {
				ifs.RxPacket, _ = strconv.ParseUint(item[2], 0, 64)
				ifs.RxBytes, _ = strconv.ParseUint(item[4], 0, 64)
			} else {
				ifs.RxError, _ = strconv.ParseUint(item[2], 0, 64)
				ifs.RxDrop, _ = strconv.ParseUint(item[4], 0, 64)
				ifs.RxOverruns, _ = strconv.ParseUint(item[6], 0, 64)
				ifs.RxFrame, _ = strconv.ParseUint(item[8], 0, 64)
			}
		case "TX":
			if item[1] == "packets" {
				ifs.TxPacket, _ = strconv.ParseUint(item[2], 0, 64)
				ifs.TxBytes, _ = strconv.ParseUint(item[4], 0, 64)
			} else {
				ifs.TxError, _ = strconv.ParseUint(item[2], 0, 64)
				ifs.TxDrop, _ = strconv.ParseUint(item[4], 0, 64)
				ifs.TxOverruns, _ = strconv.ParseUint(item[6], 0, 64)
				ifs.TxCarrier, _ = strconv.ParseUint(item[8], 0, 64)
				ifs.TxCollisions, _ = strconv.ParseUint(item[10], 0, 64)
			}
		}
	}
	// fmt.Println(*ifs)
	return ifs
}

func collectStats(name string) ([]*IfStats, error) {
	if name == "" {
		output, err := exec.Command("ifconfig").Output()
		if err != nil {
			return nil, err
		}
		iflist := strings.Split(string(output), "\n\n")
		ifstats := make([]*IfStats, 0, len(iflist))
		for _, ifentry := range iflist {
			if e := newIfStats(ifentry); e != nil {
				ifstats = append(ifstats, e)
			}
		}
		// fmt.Println(ifstats)
		if len(ifstats) > 0 {
			return ifstats, nil
		}
		return nil, fmt.Errorf("no entry")
	}
	args := []string{name}
	output, err := exec.Command("ifconfig", args...).Output()
	if err != nil {
		return nil, err
	}
	ifentry := string(output)
	e := newIfStats(ifentry)
	if e != nil {
		return []*IfStats{e}, nil
	}
	return nil, fmt.Errorf("%s not found", name)
}

func marshal(s *IfStats) []byte {
	format := `
interfaces:
  interface[name=%s]:
    state:
      name: %s
      counters:
        in-pkts: %d
        in-octets: %d
        in-errors: %d
        in-discards: %d
        out-pkts: %d
        out-octets: %d
        out-errors: %d
        out-discards: %d
`
	outstr := fmt.Sprintf(format, s.Name, s.Name,
		s.RxPacket,
		s.RxBytes,
		s.RxError,
		s.RxDrop,
		s.TxPacket,
		s.TxBytes,
		s.TxError,
		s.TxDrop,
	)
	return []byte(outstr)
}

func (sinfo *IfInfo) pollStats(db *ydb.YDB, ticker *time.Ticker, done chan bool) {
	stats, _ := collectStats("")
	for _, s := range stats {
		sinfo.Ifstats[s.Name] = s
		syncPath := fmt.Sprintf("/interfaces/interface[name=%s]", s.Name)
		db.AddSyncUpdatePath(syncPath)
	}
	for {
		select {
		case <-done:
			return
		case <-ticker.C:
			stats, _ = collectStats("")
			for _, s := range stats {
				b := marshal(s)
				// fmt.Println(string(b))
				db.Write(b)
			}
		}
	}
}

func syncStats(db *ydb.YDB, name string) {
	stats, _ := collectStats(name)
	for _, s := range stats {
		b := marshal(s)
		fmt.Println(string(b))
		db.Write(b)
	}
}

func main() {
	// ydb.SetInternalLog(ydb.LogDebug)
	done := make(chan bool)
	ticker := time.NewTicker(time.Second * 5)
	// reader := bufio.NewReader(os.Stdin)
	info := &IfInfo{Ifstats: make(map[string]*IfStats)}
	db, dbclose := ydb.OpenWithTargetStruct("subsystem", info)
	defer dbclose()
	err := db.Connect("uss://openconfig", "pub")
	if err != nil {
		log.Println(err)
		return
	}
	db.Serve()

	go info.pollStats(db, ticker, done)
	for {
		// text, _ := reader.ReadString('\n')
		// if len(text) > 0 {
		// 	done <- true
		// 	break
		// }
		time.Sleep(time.Second)
	}
}