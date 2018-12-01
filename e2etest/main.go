package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"time"
)

func main() {
	ctx, cancel := context.WithTimeout(context.Background(), time.Hour)
	defer cancel()

	start := time.Now()
	c, err := startCluster(ctx)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println("\n\nStarted in", time.Since(start).String())

	defer c.universe.Close()
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt)
	go func() {
		select {
		case <-stop:
			cancel()
		case <-c.universe.Context().Done():
		}
	}()

	fmt.Printf("export KUBECONFIG=%s\n", c.cluster.Kubeconfig())
	<-ctx.Done()
}
