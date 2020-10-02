package main

import (
	"flag"
	"time"

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
	server := wsproxy.NewServer(nil)

	server.AuthHandleFunc(proxyAuth)
	go server.Start("0.0.0.0:2080")

	time.Sleep(time.Duration(5000) * time.Second)

	server.Stop()

	/*
		sock5server, _ := socks.NewSocks5Server()
		go sock5server.Start("0.0.0.0:1080")

		time.Sleep(time.Duration(5) * time.Second)

		sock5server.Stop()

		time.Sleep(time.Duration(5) * time.Second)

		sock5server.FetchTraffic("")
	*/

	/*
		flag.Parse()
		if help || len(os.Args) == 1 {
			flag.Usage()
			return
		}
	*/
}

/*
func hello(w http.ResponseWriter, req *http.Request) {
	fmt.Fprintf(w, "hello\n")
}

func headers(w http.ResponseWriter, req *http.Request) {
	for name, headers := range req.Header {
		for _, h := range headers {
			fmt.Fprintf(w, "%v: %v\n", name, h)
		}
	}
}
*/
/*
func wmain() {
	http.HandleFunc("/hello", hello)
	http.HandleFunc("/headers", headers)

	http.ListenAndServe(":80", nil)
}
*/
