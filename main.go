package main

import (
	"flag"
	"os"
	"os/signal"

	"git.superpool.io/Jackarain/wsporxy/wsproxy"
)

var (
	help bool
)

func init() {
	flag.BoolVar(&help, "help", false, "help message")
}

func proxyAuth(user, passwd string) bool {
	return true
}

func main() {
	flag.Parse()
	/*
		if help || len(os.Args) == 1 {
			flag.Usage()
			return
		}
	*/

	server := wsproxy.NewServer(nil)
	server.AuthHandleFunc(proxyAuth)

	go server.Start("0.0.0.0:2080")

	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, os.Kill)

	<-c

	server.Stop()
}
