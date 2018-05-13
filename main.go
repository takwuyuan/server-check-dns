package main

import (
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"net"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/miekg/dns"
	fastping "github.com/tatsushid/go-fastping"

	yaml "gopkg.in/yaml.v2"
)

// WholeConfig 設定ファイル全体を表す
type WholeConfig struct {
	Global  GlobalConfig  `yaml:"global"`
	Entries []EntryConfig `yaml:"entries"`
}

// GlobalConfig グローバル部分
type GlobalConfig struct {
	Key1 string `yaml:"key1"`
}

// EntryConfig エントリー部分。監視したいサーバーのドメインや候補のIPがある
type EntryConfig struct {
	Type     string   `yaml:"type"`
	Method   string   `yaml:"method"`
	Domain   string   `yaml:"domain"`
	Servers  []string `yaml:"servers"`
	Interval int      `yaml:"interval"`
}

func pingV4(ip string) bool {
	p := fastping.NewPinger()
	ra, err := net.ResolveIPAddr("ip4:icmp", ip)
	if err != nil {
		log.Println(err)
		return false
	}
	isok := false
	p.AddIPAddr(ra)

	p.OnRecv = func(addr *net.IPAddr, rtt time.Duration) {
		isok = true
	}
	err = p.Run()
	if err != nil {
		log.Println(err)
		return false
	}

	return isok
}

func updateARecord(domain string, ipA string) {
	rr, _ := dns.NewRR(fmt.Sprintf("%s. 3600 IN A %s", domain, ipA))
	rrx := rr.(*dns.A)
	dns.HandleFunc(domain, func(w dns.ResponseWriter, r *dns.Msg) {
		m := new(dns.Msg)
		m.SetReply(r)
		m.Authoritative = true
		m.Ns = []dns.RR{rrx}
		w.WriteMsg(m)
	})
}

func healthPingCheck(entry EntryConfig) string {

	for _, ip := range entry.Servers {
		if pingV4(ip) {
			return ip
		}
	}
	return ""

}

func main() {
	configFile := ""
	flag.StringVar(&configFile, "config", "", "config file")
	flag.Parse()
	if configFile == "" {
		log.Println("no config flie")
		return
	}

	buf, err := ioutil.ReadFile(configFile)
	if err != nil {
		panic(err)
	}

	var d WholeConfig
	err = yaml.Unmarshal(buf, &d)
	if err != nil {
		panic(err)
	}

	var zoneIP = make(map[string]string)

	for _, entry := range d.Entries {
		ip := healthPingCheck(entry)
		if ip != "" {
			updateARecord(entry.Domain, ip)
			zoneIP[entry.Domain] = ip
		} else {
			zoneIP[entry.Domain] = ""
		}
	}

	log.Println("now serving")
	go func() {
		srv := &dns.Server{Addr: ":8053", Net: "udp"}
		if err := srv.ListenAndServe(); err != nil {
			log.Fatalf("Failed to set udp listener %s\n", err.Error())
		}
	}()

	go func() {
		srv := &dns.Server{Addr: ":8053", Net: "tcp"}
		if err := srv.ListenAndServe(); err != nil {
			log.Fatalf("Failed to set tcp listener %s\n", err.Error())
		}
	}()

	t := time.NewTicker(time.Second)
	tick := 3601
	go func() {
		for {
			select {
			case <-t.C:

				for _, entry := range d.Entries {
					if tick%entry.Interval == 0 {
						// log.Printf("tick %s %s\n", entry.Domain, zoneIP[entry.Domain])

						ip := healthPingCheck(entry)
						if ip != zoneIP[entry.Domain] {
							// log.Printf("update %s %s\n", entry.Domain, ip)
							updateARecord(entry.Domain, ip)
						}
						zoneIP[entry.Domain] = ip
					}
				}
				tick--
				if tick == 0 {
					tick = 3601
				}
			}
		}
	}()

	sig := make(chan os.Signal)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
	s := <-sig
	log.Fatalf("Signal (%v) received, stopping\n", s)
}
