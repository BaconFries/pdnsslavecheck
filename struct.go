package main

type tomlConfig struct {
	Title		string
	Powerdns	powerdns
	Slaves		slaves
	Redisconf	redisconf
}

type powerdns struct {
	API	string
	APIkey	string
	Master	string
}

type slaves struct {
	Servers	[]string
}

type redisconf struct {
	Keyprefix	string
	Server		string
	Password	string
	DB		int
	Expire_sec	int
}

type Input struct {
	NotifiedSerial	int	`json:"notified_serial"`
	Serial		int	`json:"serial"`
	Account		string	`json:"account"`
	Name		string	`json:"name"`
	Kind		string	`json:"kind"`
	Masters		[]string	`json:"masters"`
	URL		string	`json:"url"`
	DNSsec		bool	`json:"dnssec"`
	ID		string	`json:"id"`
	LastCheck	int	`json:"last_check"`
}      

type Output struct {
	Master_serial	uint32		`json:"master_serial"`
	Percent		float64		`json:"percent"`
	Count		int		`json:"count"`
	Domain		string		`json:"domain"`
	Nameservers	[]Nameservers	`json:"nameservers"`
}

type Nameservers struct {
	Nameserver	string	`json:"nameserver"`
	Serial		uint32	`json:"serial"`
	Match_master	bool	`json:"match_master"`
}