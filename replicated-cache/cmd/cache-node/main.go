/*
Author: Phyu Lwin
Date: 2025 Aug 10th
Project: Replicated In-Memory Cache (Golang)

This file implements the main entry point for a cache node in a replicated in-memory cache cluster.
It handles command-line arguments, initializes the cache node, sets up HTTP routes, and manages
the node's lifecycle including heartbeat and janitor routines.
*/

package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log"
	"math/rand/v2"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"time"

	"github.com/you/replicated-cache/internal/cache"
)

func main() {
	var (
		addr   = flag.String("addr", ":8081", "listen address")
		peers  = flag.String("peers", "", "comma-separated peer base URLs (e.g. http://localhost:8082,http://localhost:8083)")
		idFlag = flag.String("id", "", "node id (defaults to addr+rand)")
		hb     = flag.Duration("hb", 5*time.Second, "heartbeat interval")
		reqTO  = flag.Duration("req-timeout", 4*time.Second, "replication request timeout")
	)
	flag.Parse()

	id := *idFlag
	if id == "" {
		id = fmt.Sprintf("%s#%04x", *addr, rand.Uint32())
	}
	var peerList []string
	if *peers != "" {
		peerList = strings.Split(*peers, ",")
	}

	node := cache.NewNode(id, *addr, peerList)
	node.HBInterval = *hb
	node.ReqTimeout = *reqTO

	srv := &http.Server{
		Addr:              *addr,
		Handler:           node.Routes(),
		ReadHeaderTimeout: 5 * time.Second,
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	go node.HeartbeatLoop(ctx)
	go node.JanitorLoop(ctx)

	log.Printf("node %q listening on %s; peers=%v", node.ID, *addr, peerList)
	if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		log.Fatalf("server error: %v", err)
	}

	<-ctx.Done()
	shCtx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	_ = srv.Shutdown(shCtx)
}