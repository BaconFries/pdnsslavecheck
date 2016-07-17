# pdnsslavecheck
Compare serial number of slave servers with master PowerDNS server.
The results are stored in redis for easy retrival. 

## Configuration
The config file can be place with the program or in more standard locations such as /etc or /usr/local/etc.

```toml
[powerdns]
api = "https://powerdns.server/api/v1/servers/localhost/zones"
apikey = "p4ssw0rd"
master = "10.20.30.40"

[slaves]
servers = [ "1.1.1.1", "2.2.2.2" ]

[redisconf]
keyprefix = "slavecheck"
server = "redis.server:6379"
password = ""
db = 0
expire_sec = 120
```


#### Redis Data
```json
{
	"notified_serial" : 2016070805,
	"serial" : 2016070805,
	"account" : "",
	"name" : "example.com.",
	"kind" : "Master",
	"masters" : [],
	"url" : "api/v1/servers/localhost/zones/example.com.",
	"dnssec" : false,
	"id" : "example.com.",
	"last_check" : 0
}
```
