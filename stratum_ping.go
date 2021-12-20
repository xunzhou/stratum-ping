/*
	stratum-ping

	Copyright ©2021, 2Miners.com
*/

package stratum_ping

import (
	"bufio"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"net"
	"time"
)

type StratumPinger struct {
	Login string
	Pass  string
	Count int
	Ipv6  bool
	Host  string
	Port  string
	Addr  *net.IPAddr
	Proto string
	Tls   bool
}

func (p *StratumPinger) Do() string {
	if err := p.Resolve(); err != nil {
		return "err"
	}
	/*
		creds := ""
		if p.Proto == "stratum1" {
			creds = " with credentials: " + p.Login + ":" + p.Pass
		}
		tls := ""
		if p.Tls {
			tls = " TLS"
		}
	*/

	// fmt.Printf("PING stratum %s (%s)%s port %s%s\n", p.Host, p.Addr.String(), tls, p.Port, creds)

	min := time.Duration(time.Hour)
	max := time.Duration(0)
	avg := time.Duration(0)
	avgCount := 0
	success := 0
	res := ""
	stats := ""
	start := time.Now()

	for i := 0; i < p.Count; i++ {
		elapsed, err := p.DoPing()
		if err != nil {
			fmt.Printf("%s (%s): seq=%d, %s\n", p.Host, p.Addr.String(), i, err)
		} else {
			fmt.Printf("%s (%s): seq=%d, time=%s\n", p.Host, p.Addr.String(), i, elapsed.String())
			if elapsed > max {
				max = elapsed
			}
			if elapsed < min {
				min = elapsed
			}
			avg += elapsed
			avgCount++
			success++
		}
		time.Sleep(1 * time.Second)
	}
	// fmt.Printf("\n--- %s ping statistics ---\n", p.Host)
	loss := 100 - int64(float64(success)/float64(p.Count)*100.0)
	duration := time.Since(start)
	stats = fmt.Sprintf("%d packets transmitted, %d received, %d%% packet loss, time %s\n", p.Count, success, loss, duration)
	// fmt.Print(stats)
	if success > 0 {
		res = fmt.Sprintf("min/avg/max = %s, %s, %s\n", min.String(), (avg / time.Duration(avgCount)).String(), max.String())
		// fmt.Println(res)
	}

	return (stats + res)
}

func (p *StratumPinger) Resolve() error {
	var err error
	network := "ip4"

	if p.Ipv6 {
		network = "ip6"
	}

	p.Addr, err = net.ResolveIPAddr(network, p.Host)
	if err != nil {
		return fmt.Errorf("failed to resolve host name: %s", err)
	}
	return nil
}

func (p *StratumPinger) DoPing() (time.Duration, error) {
	var dial string
	var network string

	if p.Ipv6 {
		network = "tcp6"
		dial = "[" + p.Addr.IP.String() + "]:" + p.Port
	} else {
		network = "tcp4"
		dial = p.Addr.IP.String() + ":" + p.Port
	}

	var err error
	var conn net.Conn
	if p.Tls {
		cfg := &tls.Config{InsecureSkipVerify: true}
		conn, err = tls.Dial(network, dial, cfg)
	} else {
		conn, err = net.Dial(network, dial)
	}
	if err != nil {
		return 0, err
	}

	enc := json.NewEncoder(conn)
	buff := bufio.NewReaderSize(conn, 1024)

	readTimeout := 10 * time.Second
	writeTimeout := 10 * time.Second

	conn.SetWriteDeadline(time.Now().Add(writeTimeout))

	var req map[string]interface{}

	switch p.Proto {
	case "stratum1":
		req = map[string]interface{}{"id": 1, "jsonrpc": "2.0", "method": "eth_submitLogin", "params": []string{p.Login, p.Pass}}
	case "stratum2":
		req = map[string]interface{}{"id": 1, "method": "mining.subscribe", "params": []string{"stratum-ping/1.0.0", "EthereumStratum/1.0.0"}}
	}

	start := time.Now()
	if err = enc.Encode(&req); err != nil {
		return 0, err
	}
	conn.SetReadDeadline(time.Now().Add(readTimeout))
	if _, _, err = buff.ReadLine(); err != nil {
		return 0, err
	}
	elapsed := time.Since(start)
	conn.Close()

	return elapsed, nil
}
