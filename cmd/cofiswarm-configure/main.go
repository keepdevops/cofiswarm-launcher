package main

import (
	"context"
	"flag"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"github.com/keepdevops/cofiswarm-launcher/internal/bus"
	"github.com/keepdevops/cofiswarm-launcher/internal/configure"
	"github.com/keepdevops/cofiswarm-observer-sdk/pkg/buspresence"
	"github.com/keepdevops/cofiswarm-observer-sdk/pkg/servicecomponent"
)

func main() {
	addr := flag.String("listen", ":8017", "configure API listen address (HTTP mode)")
	busMode := flag.Bool("bus", false, "serve .launcher.* on the NATS observer bus instead of HTTP")
	natsURL := flag.String("nats", "nats://127.0.0.1:4222", "NATS URL (bus mode)")
	flag.Parse()

	srv := configure.NewServer()

	if *busMode {
		serveBus(*natsURL, srv)
		return
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	// Carrier presence (broker-free, default-off via COFISWARM_BRIDGE_URL): appear in the
	// observer live roster over the zmq-bridge without needing a NATS broker.
	stopPresence := buspresence.StartPresence(os.Getenv("COFISWARM_BRIDGE_URL"), "launcher", map[string]any{"name": "launcher"})
	defer stopPresence()

	httpSrv := &http.Server{Addr: *addr, Handler: srv.Handler()}
	go func() {
		log.Printf("configure API on %s state=%s", *addr, srv.StateDir)
		if err := httpSrv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("configure API serve: %v", err)
		}
	}()

	<-ctx.Done()
	log.Print("configure API stopping")
	if err := httpSrv.Shutdown(context.Background()); err != nil {
		log.Printf("configure API shutdown: %v", err)
	}
}

func serveBus(url string, srv *configure.Server) {
	nc, err := servicecomponent.Connect(url, "cofiswarm-launcher")
	if err != nil {
		log.Fatalf("bus connect %s: %v", url, err)
	}
	defer nc.Close()
	comp := servicecomponent.New(nc, "launcher", "launcher", bus.Routes(srv))
	if err := comp.Start(); err != nil {
		log.Fatalf("bus start: %v", err)
	}
	defer comp.Shutdown()
	log.Printf("launcher on bus %s (.launcher.configure/.launcher.status) state=%s", url, srv.StateDir)

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
	<-sig
	log.Print("launcher bus stopping")
}
