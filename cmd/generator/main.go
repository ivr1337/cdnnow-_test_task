package main

import (
	"context"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"sync/atomic"
	"syscall"
	"time"
)

const (
	workers   = 20
	targetURL = "http://localhost:8080/calc"
)

var (
	success atomic.Uint64
	failed  atomic.Uint64
)

func worker(ctx context.Context, client *http.Client, wg *sync.WaitGroup) {
	defer wg.Done()

	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		num := rand.Intn(201) - 100
		url := fmt.Sprintf("%s?num=%d", targetURL, num)

		request, err := http.NewRequestWithContext(
			ctx,
			http.MethodPost,
			url,
			http.NoBody,
		)
		if err != nil {
			failed.Add(1)
			continue
		}

		response, err := client.Do(request)
		if err != nil {
			failed.Add(1)
			continue
		}

		_, readErr := io.Copy(io.Discard, response.Body)
		response.Body.Close()

		if response.StatusCode == http.StatusOK && readErr == nil {
			success.Add(1)
		} else {
			failed.Add(1)
		}
	}
}

func main() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	client := &http.Client{
		Timeout: 5 * time.Second,
	}

	var wg sync.WaitGroup

	for range workers {
		wg.Add(1)
		go worker(ctx, client, &wg)
	}

	fmt.Printf("%d workers started\n", workers)

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT)

	<-sig
	cancel()

	wg.Wait()

	fmt.Printf(
		"\nsuccess=%d failed=%d",
		success.Load(),
		failed.Load(),
	)
}
