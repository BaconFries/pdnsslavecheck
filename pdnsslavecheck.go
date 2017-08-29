package main

import (
	"os"
	"time"
	"log"
	"errors"
	"net/http"
	"crypto/tls"
	"io/ioutil"
	"github.com/BurntSushi/toml"
	"github.com/miekg/dns"
	"github.com/bsm/redis-lock"
	"github.com/go-redis/redis"
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

func getstate(rkey string, redisclient *redis.Client) (result Output, err error )  {
        state, err1     := redisclient.Get(rkey).Result()
        if err1 != nil {
		err = errors.New("Not yet implemented")
                return
        }
        bytes := []byte(state)
        json.Unmarshal(bytes, &result)
	return result, nil
}

func findlast(ns Output, slaveserver string) int {
	for index, ser := range ns.Nameservers {
		if (ser.Nameserver == slaveserver) {
			return index
		}
	}
	return -1
}

func checksoa(wg *sync.WaitGroup, name string, keyprefix string, slaveservers []string, redisclient *redis.Client, expire time.Duration) {
	defer wg.Done()
	count           := 0
	total           := 0
	qname           := chop(name)
	now             := time.Now()
	rkey            := config.Redisconf.Keyprefix + ":" + qname + "."
	state,state_err	:= getstate(rkey, redisclient)
	ms              := getserial( qname, config.Powerdns.Master )
	o               := Output{}
	o.Domain        =  qname
	o.LastCheck     = int32(now.Unix())
	o.Master_serial =  ms
	swg             := new(sync.WaitGroup)
	for i := range slaveservers {
		swg.Add(1)
		slaveserver := slaveservers[i]
		laststate   := -1
        	if (state_err == nil) {
			laststate = findlast(state, slaveserver)
        	}
		var ds uint32
		var lc int32
		if ( laststate != -1 )  {
			lc = int32(state.Nameservers[laststate].LastCheck) + int32(300)
			ds = uint32(state.Nameservers[laststate].Serial)
		}
		go func() {
			defer swg.Done()
			if ( ms != ds || lc < o.LastCheck) {
				ds = getserial( qname, slaveserver )
				lc = o.LastCheck
			}
			n            := Nameservers{}
			n.Nameserver =  slaveserver
			n.Serial     =  ds
			n.LastCheck  =  lc
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
	percent	  := (float64(count) / float64(total) * 100)
	o.Count	  =  count
	o.Total	  =  total
	o.Percent =  percent
	json, _   := json.Marshal(o)
	err	      := redisclient.Set(rkey, json, expire).Err()
	if err != nil {
		log.Println(err)
	}
}

var wg		sync.WaitGroup
var input	[]Input
var config	tomlConfig
var slaveservers 	[]string
func main() {
	// Read config file
	configfile := "pdnsslavecheck.cfg"
	if _, err := os.Stat("/etc/pdnsslavecheck.cfg"); err == nil {
		configfile = "/etc/pdnsslavecheck.cfg"
	} else if _, err := os.Stat("/usr/local/etc/pdnsslavecheck.cfg"); err == nil {
		configfile = "/usr/local/etc/pdnsslavecheck.cfg"
	}
	
	if _, err := toml.DecodeFile(configfile, &config); err != nil {
		log.Fatal(err)
		return
	}

	// Setup Redis Key expire time
	expire := time.Duration(config.Redisconf.Expire_sec) * time.Second
	delay  := time.Duration(config.Redisconf.Delay_sec) * time.Second
	
	// Connect to Redis DB
	redisclient := redis.NewClient(&redis.Options{
			Addr:     config.Redisconf.Server,
			Password: config.Redisconf.Password,
			DB:       config.Redisconf.DB,
	})

	// Obtain a new lock with default settings
	lock, err := lock.ObtainLock(redisclient, "lock.slavecheck", &lock.LockOptions{
                LockTimeout: delay,
        })
	if err != nil {
		os.Exit(0)
		return
	} else if lock == nil {
		os.Exit(0)
		return
	}
	
	defer lock.Unlock()
	
	slaveservers = config.Slaves.Servers
	keyprefix    := config.Redisconf.Keyprefix
	
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
	
	// Check each domain in zone list
	jsonbytes := []byte(zonelist)
	json.Unmarshal(jsonbytes, &input)
	for i := range input {
		wg.Add(1)
		go checksoa(&wg, input[i].Name, keyprefix, slaveservers, redisclient, expire)
	}
	ok, err := lock.Lock()
	if err != nil {
		return
	} else if !ok {
		return
	}
	wg.Wait()
	os.Exit(0)
}
