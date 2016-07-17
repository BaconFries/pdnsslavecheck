package main

import (
	"os"
	"time"
	"log"
	"net/http"
	"crypto/tls"
	"io/ioutil"
	"github.com/BurntSushi/toml"
	"github.com/miekg/dns"
	"gopkg.in/redis.v4"
	"encoding/json"
	"sync"
)

func getserial(target string, server string) uint32 {
	c := dns.Client{}
	m := dns.Msg{}
	m.SetQuestion(target+".", dns.TypeSOA)
	r,_, err := c.Exchange(&m, server+":53")
	if err != nil {
		return 0
	}
	if len(r.Answer) == 0 {
		return 0
	}
	SOArecord := r.Answer[0].(*dns.SOA)
	return SOArecord.Serial
} 

func chop(s string) string {
	return s[:len(s)-1]
}

func checksoa(wg *sync.WaitGroup, name string, config tomlConfig, redisclient *redis.Client, expire time.Duration) {
	defer wg.Done()
	count		:= 0
	total		:= 0
	qname		:= chop(name)
	ms		:= getserial( qname, config.Powerdns.Master )
	o		:= Output{}
	o.Domain	=  qname
	o.Master_serial	=  ms
	swg := new(sync.WaitGroup)
	for _, slaveserver := range config.Slaves.Servers {
		swg.Add(1)
		go func() {
			defer swg.Done()
			ds		:= getserial( qname, slaveserver )
			n		:= Nameservers{}
			n.Nameserver	=  slaveserver
			n.Serial 	=  ds
			if ms == ds {
				count++
				n.Match_master = true
			}else{
				n.Match_master = false
			}
			o.Nameservers = append(o.Nameservers, n)
			total++
		}()
	}
	swg.Wait()
	percent		:= (float64(count) / float64(total) * 100)
	o.Count		=  count
	o.Percent	=  percent
	json, _ 	:= json.Marshal(o)
	rkey		:= config.Redisconf.Keyprefix + ":" + qname + "."
	err		:= redisclient.Set(rkey, json, expire).Err()
	if err != nil {
		log.Println(err)
	}
}

var wg		sync.WaitGroup
var input	[]Input
var config	tomlConfig

func main() {
	// Read config file
	configfile := "pdnsslavecheck.cfg"
	if _, err := os.Stat("/etc/pdnsslavecheck.cfg"); err == nil {
		configfile = "/etc/pdnsslavecheck.cfg"
	}
	if _, err := os.Stat("/usr/local/etc/pdnsslavecheck.cfg"); err == nil {
		configfile = "/usr/local/etc/pdnsslavecheck.cfg"
	}
	
	if _, err := toml.DecodeFile(configfile, &config); err != nil {
		log.Fatal(err)
		return
	}
	if cap(config.Slaves.Servers) == 0 {
		log.Fatal("No slave servers defined in config")
	}

	// Download zone list from PowerDNS
	cfg := &tls.Config{
		InsecureSkipVerify: true,
	}

	http.DefaultClient.Transport = &http.Transport{
		TLSClientConfig: cfg,
	}

	req, err := http.NewRequest("GET", config.Powerdns.API, nil)
	if err != nil {
			log.Fatal(err)
	}
	req.Header.Set("X-API-Key", config.Powerdns.APIkey)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		log.Fatal(err)
	}
	defer resp.Body.Close()
	zonelist, err := ioutil.ReadAll(resp.Body)
	if err != nil {
			log.Fatal(err.Error())
	}
	
	// Setup Redis Key expire time
	expire := time.Duration(config.Redisconf.Expire_sec) * time.Second
	
	// Connect to Redis DB
	redisclient := redis.NewClient(&redis.Options{
			Addr:     config.Redisconf.Server,
			Password: config.Redisconf.Password,
			DB:       config.Redisconf.DB,
	})

	// Check each domain in zone list
	jsonbytes := []byte(zonelist)
	json.Unmarshal(jsonbytes, &input)
	for i := range input {
		wg.Add(1)
		go checksoa(&wg, input[i].Name, config, redisclient, expire)
	}
	
	wg.Wait()
	os.Exit(0)
}

