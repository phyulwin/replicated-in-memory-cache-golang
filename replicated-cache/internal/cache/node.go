/*
Author: phyu lwin
Project: replicated-in-memory-cache-golang
Date: Aug 10th 2025

Summary:
This file defines the Node type, which represents a single node in a replicated in-memory cache cluster. 
The Node manages peer discovery, health checking, replication of cache updates, and periodic cleanup of expired entries.
It handles communication with peer nodes over HTTP, tracks peer health, and coordinates data consistency across the cluster.

Functions in this file:
- NewNode: Constructs a new Node with the given ID, address, and initial peers.
- Store: Returns the underlying Store instance for this Node.
- activePeers: Returns a slice of currently active peer addresses.
- bumpFail: Updates failure counts for a peer and removes it if failures exceed a threshold.
- HeartbeatLoop: Periodically checks the health of peer nodes and updates their status.
- JanitorLoop: Periodically removes expired tombstoned entries from the store.
- Replicate: Sends a synchronization message to peers and waits for acknowledgements.
*/

package cache

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"
)

type Node struct {
	ID     string
	Addr   string
	store  *Store
	client *http.Client

	peersMu     sync.RWMutex
	peers       map[string]struct{}
	failCounts  map[string]int
	maxFailures int

	ReqTimeout   time.Duration
	HBInterval   time.Duration
	JanitorEvery time.Duration
	TombstoneTTL time.Duration
}

func NewNode(id, addr string, initialPeers []string) *Node {
	n := &Node{
		ID:           id,
		Addr:         addr,
		store:        NewStore(),
		client:       &http.Client{Timeout: 5 * time.Second},
		peers:        make(map[string]struct{}),
		failCounts:   make(map[string]int),
		maxFailures:  3,
		ReqTimeout:   4 * time.Second,
		HBInterval:   5 * time.Second,
		JanitorEvery: 2 * time.Second,
		TombstoneTTL: 5 * time.Minute,
	}
	for _, p := range initialPeers {
		p = strings.TrimRight(strings.TrimSpace(p), "/")
		if p != "" {
			n.peers[p] = struct{}{}
		}
	}
	return n
}

func (n *Node) Store() *Store { return n.store }

func (n *Node) activePeers() []string {
	n.peersMu.RLock()
	defer n.peersMu.RUnlock()
	out := make([]string, 0, len(n.peers))
	for p := range n.peers {
		out = append(out, p)
	}
	return out
}
func (n *Node) bumpFail(p string, ok bool) {
	n.peersMu.Lock()
	defer n.peersMu.Unlock()
	if ok {
		n.failCounts[p] = 0
		return
	}
	n.failCounts[p]++
	if n.failCounts[p] >= n.maxFailures {
		delete(n.peers, p)
		log.Printf("[peers] %s exceeded failures; removing", p)
	}
}

func (n *Node) HeartbeatLoop(ctx context.Context) {
	t := time.NewTicker(n.HBInterval)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			for _, p := range n.activePeers() {
				req, _ := http.NewRequestWithContext(ctx, http.MethodGet, p+"/health", nil)
				resp, err := n.client.Do(req)
				if err != nil || resp.StatusCode != 200 {
					if resp != nil {
						resp.Body.Close()
					}
					n.bumpFail(p, false)
					continue
				}
				io.Copy(io.Discard, resp.Body)
				resp.Body.Close()
				n.bumpFail(p, true)
			}
		}
	}
}

func (n *Node) JanitorLoop(ctx context.Context) {
	t := time.NewTicker(n.JanitorEvery)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			n.store.HardDeleteExpired(time.Now(), n.TombstoneTTL)
		}
	}
}

// Replicate sends a SyncMsg to peers and waits for min/full acknowledgements.
func (n *Node) Replicate(ctx context.Context, msg SyncMsg, min int, full bool) (acked, total int, err error) {
	peers := n.activePeers()
	total = len(peers)
	if total == 0 {
		if min > 0 || full {
			return 0, 0, fmt.Errorf("no peers available")
		}
		return 0, 0, nil
	}

	target := min
	if full {
		target = total
	}
	if target < 0 {
		target = 0
	}
	if target > total {
		target = total
	}

	ctx, cancel := context.WithTimeout(ctx, n.ReqTimeout)
	defer cancel()

	payload, _ := json.Marshal(msg)
	type res struct{ ok bool; err error }
	ch := make(chan res, total)

	for _, p := range peers {
		go func(peer string) {
			req, _ := http.NewRequestWithContext(ctx, http.MethodPost, peer+"/sync", bytes.NewReader(payload))
			req.Header.Set("Content-Type", "application/json")
			resp, e := n.client.Do(req)
			if e != nil {
				n.bumpFail(peer, false)
				ch <- res{false, e}
				return
			}
			io.Copy(io.Discard, resp.Body)
			resp.Body.Close()
			if resp.StatusCode/100 == 2 {
				n.bumpFail(peer, true)
				ch <- res{true, nil}
				return
			}
			n.bumpFail(peer, false)
			ch <- res{false, fmt.Errorf("status %d", resp.StatusCode)}
		}(p)
	}

	var firstErr error
	for acked < target {
		select {
		case <-ctx.Done():
			if firstErr == nil {
				firstErr = fmt.Errorf("timeout waiting for %d/%d acks (got %d)", target, total, acked)
			}
			return acked, total, firstErr
		case r := <-ch:
			if r.ok {
				acked++
			} else if firstErr == nil {
				firstErr = r.err
			}
		}
	}
	return acked, total, firstErr
}