package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"

	"gitee.com/jackarain/wsproxy/wsproxy"
)

var (
	help     bool
	config   string
	bindaddr string
)

func init() {
	flag.BoolVar(&help, "help", false, "help message")
	flag.StringVar(&config, "config", "", "json config file")
	flag.StringVar(&bindaddr, "addr", "0.0.0.0:2080", "proxy service address")
}

func proxyAuth(user, passwd string) bool {
	// 认证用户名和密码.
	v, found := wsproxy.Users[user]
	if !found {
		return false
	}
	if v != passwd {
		return false
	}

	return true
}

func main() {
	path, err := os.Getwd()
	if err != nil {
		log.Println(err)
	}
	fmt.Println("Current directory:", path)

	flag.Parse()

	if help {
		flag.Usage()
		return
	}

	if config != "" {
		wsproxy.JSONConfig = config
	}

	server := wsproxy.NewServer(nil)
	server.AuthHandleFunc(proxyAuth)

	go server.Start(bindaddr)

	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, os.Kill)

	<-c

	server.Stop()
}
