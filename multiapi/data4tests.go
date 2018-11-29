package multiapi

import . "github.com/iotaledger/iota.go/trinary"

var endpoints = []string{
	"https://nodes.thetangle.org:443",
	"http://node.iotalt.com:14600",
	"https://field.deviota.com:443",
	"https://potato.iotasalad.org:14265",
}

var txs = Hashes{
	Trytes("ANAJPUJTQEZBQILHHGEAOJGIBREGNCZNAMWIJXQMIQKTIIHBFFPUKGOVPYB9BGYPTASKRYQXFUAUZ9999"),
	Trytes("ZJZFYYJXIOGPRN9USXAZIXWJDEJRIJEHNBLRFWJFNWXDWAPIBJE9ZMQLZDDVFBJRBNACKFJWX9EOZ9999"),
	Trytes("TTIQHMOJKQEDBOBASIROLCGBEXPCNVLUHAAOKSMEWIUNAYNMT9QIZTVTWCSYXOZNGGPNMQ9IMTMNA9999"),
}
