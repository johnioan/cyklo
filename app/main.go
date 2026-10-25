package main

import (
	"context"
	"embed"
	"flag"
	"fmt"
	"io/fs"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
)

//go:embed web
var webFS embed.FS

var version = "0.1.0"

func envOr(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}

func main() {
	port := flag.String("port", envOr("CYKLO_PORT", "8765"), "TCP port")
	data := flag.String("data", envOr("CYKLO_DATA", "./data"), "data folder")
	addr := flag.String("addr", envOr("CYKLO_ADDR", "0.0.0.0"), "listen address")
	flag.Parse()

	st, err := OpenStore(*data)
	if err != nil {
		log.Fatalf("cannot open data: %v", err)
	}
	n := NewNotifier(st)
	api := &API{st: st, n: n}

	mux := http.NewServeMux()
	api.routes(mux)
	mux.HandleFunc("GET /api/auth/check", api.authCheck)
	mux.HandleFunc("GET /api/version", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, 200, map[string]string{"version": version})
	})
	sub, _ := fs.Sub(webFS, "web")
	files := http.FileServerFS(sub)
	mux.HandleFunc("GET /", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-cache")
		files.ServeHTTP(w, r)
	})

	srv := &http.Server{
		Addr:              fmt.Sprintf("%s:%s", *addr, *port),
		Handler:           api.requireAuth(mux),
		ReadHeaderTimeout: 10 * time.Second,
	}
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	go n.Run(ctx)
	go func() {
		<-ctx.Done()
		c, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = srv.Shutdown(c)
	}()
	log.Printf("Cyklo %s listening on %s, data in %s", version, srv.Addr, *data)
	if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatal(err)
	}
}
