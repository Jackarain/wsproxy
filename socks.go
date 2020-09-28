package main

import (
	"flag"
	"sync"
	"time"

	"git.superpool.io/Jackarain/wsporxy/socks"
)

var (
	help bool

	speedLimit sync.Map
)

func init() {
	flag.BoolVar(&help, "help", false, "help message")
}

func main() {
	sock5server, _ := socks.NewSocks5Server()
	go sock5server.Start("0.0.0.0:1080")

	time.Sleep(time.Duration(5) * time.Second)

	sock5server.Stop()

	time.Sleep(time.Duration(5) * time.Second)

	sock5server.FetchTraffic("")

	/*
		flag.Parse()
		if help || len(os.Args) == 1 {
			flag.Usage()
			return
		}
	*/
}
