package main

/*
  #cgo CFLAGS: -I${SRCDIR}/../../example/test_golang_14/c_lib
  #cgo LDFLAGS: -L${SRCDIR}/../../example/test_golang_14 -lcalculator -lcalculator_rust

  #include <stdint.h>
  #include "calculator.h"

  int64_t sub(int64_t a, int64_t b);
*/
import "C"
import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"sync"
	"syscall"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

var a, b int64
var mu sync.Mutex

func add(a, b int64) int64 {
	return int64(C.add(C.int64_t(a), C.int64_t(b)))
}

func sub(a, b int64) int64 {
	return int64(C.sub(C.int64_t(a), C.int64_t(b)))
}

func calcHandler(w http.ResponseWriter, r *http.Request) {
	recordRequest()
	if r.Method != "POST" {
		http.Error(w, "no no no", http.StatusMethodNotAllowed)
		return
	}
	rawNum := r.URL.Query().Get("num")
	if rawNum == "" {
		http.Error(w, "nil nil nil", http.StatusBadRequest)
		return
	}

	num, err := strconv.ParseInt(rawNum, 10, 64)
	if err != nil {
		http.Error(w, fmt.Sprintf("bad value: %v", err), http.StatusBadRequest)
		return
	}
	mu.Lock()
	cStarted := time.Now()
	a = add(a, num)
	cDuration := time.Since(cStarted)
	rustStarted := time.Now()
	b = sub(b, num)
	rustDuration := time.Since(rustStarted)
	mu.Unlock()
	cCallDuration.Observe(cDuration.Seconds())
	rustCallDuration.Observe(rustDuration.Seconds())
	w.WriteHeader(200)
	_, _ = w.Write([]byte("ok"))
	return
}
func tickerPrinter(ctx context.Context) {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			printer("periodic")
		}
	}
}
func printer(label string) {
	mu.Lock()
	defer mu.Unlock()
	fmt.Println(fmt.Sprintf("[%v] sum=%v sub=%v", label, a, b))
}
func main() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT)
	defer signal.Stop(sig)
	go func() {
		<-sig
		fmt.Println("\nSIGINT received, shutting down...")
		cancel()
	}()

	go tickerPrinter(ctx)

	http.HandleFunc("/calc", calcHandler)
	metricsRegistry := prometheus.NewRegistry()
	metricsRegistry.MustRegister(cCallDuration, rustCallDuration, httpRPS)
	prometheusHandler := promhttp.HandlerFor(
		metricsRegistry,
		promhttp.HandlerOpts{},
	)
	http.HandleFunc("/metrics", func(w http.ResponseWriter, r *http.Request) {
		updateRPSMetric()
		prometheusHandler.ServeHTTP(w, r)
	})
	server := &http.Server{
		Addr: ":8080",
	}
	go func() {
		err := server.ListenAndServe()
		if err != nil && err != http.ErrServerClosed {
			log.Fatal(err)
		}
	}()
	<-ctx.Done()
	printer("final")
	if err := server.Close(); err != nil {
		log.Fatal(err)
	}
}
