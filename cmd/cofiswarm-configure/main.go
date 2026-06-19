package main

import (
	"flag"
	"log"
	"net/http"

	"github.com/keepdevops/cofiswarm-launcher/internal/configure"
)

func main() {
	addr := flag.String("listen", ":8017", "configure API listen address")
	flag.Parse()
	srv := configure.NewServer()
	log.Printf("configure API on %s state=%s", *addr, srv.StateDir)
	log.Fatal(http.ListenAndServe(*addr, srv.Handler()))
}
