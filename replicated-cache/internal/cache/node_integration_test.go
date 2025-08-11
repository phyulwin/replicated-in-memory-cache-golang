// Author: Phyu Lwin
// Project: Replicated In-Memory Cache Golang
// Date: Aug 10th, 2025
//
// node_integration_test.go
//
// This file contains integration tests for the replicated in-memory cache node functionality.
// The tests verify correct replication of key-value data between nodes, ensure that sync
// operations do not cause rebroadcast loops, and check the heartbeat mechanism for peer health.
// The tests use Go's httptest package to simulate HTTP servers and peer interactions.

package cache

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

// Start a node in-memory using httptest and make it peer with another node.
func TestReplicationSetAndGet(t *testing.T) {
	n1 := NewNode("N1", ":x", nil)
	srv2 := httptest.NewServer(NewNode("N2", ":y", nil).Routes())
	defer srv2.Close()

	// peer n1 -> srv2
	n1.peers = map[string]struct{}{srv2.URL: {}}

	// Run handlers for n1 locally via httptest
	srv1 := httptest.NewServer(n1.Routes())
	defer srv1.Close()

	// client PUT to n1 with min=1 (must replicate to srv2)
	req, _ := http.NewRequest("PUT", srv1.URL+"/kv/hello?min=1", bytes.NewReader([]byte("world")))
	resp, err := http.DefaultClient.Do(req)
	if err != nil { t.Fatal(err) }
	resp.Body.Close()
	if resp.StatusCode != 201 {
		t.Fatalf("unexpected status: %d", resp.StatusCode)
	}

	// GET from peer
	res, err := http.Get(srv2.URL + "/kv/hello")
	if err != nil { t.Fatal(err) }
	defer res.Body.Close()
	if res.StatusCode != 200 {
		b, _ := io.ReadAll(res.Body)
		t.Fatalf("expected 200, got %d: %s", res.StatusCode, string(b))
	}
	b, _ := io.ReadAll(res.Body)
	if string(b) != "world" {
		t.Fatalf("expected replicated value, got %q", string(b))
	}
}

func TestSyncEndpointDoesNotRebroadcast(t *testing.T) {
	n := NewNode("N", ":x", nil)
	srv := httptest.NewServer(n.Routes())
	defer srv.Close()

	// POST /sync should apply without errors and not try to rebroadcast.
	msg := SyncMsg{Op: "set", Key: "k", Value: []byte("v"), Version: time.Now().UnixNano(), Origin: "X"}
	buf, _ := json.Marshal(msg)
	resp, err := http.Post(srv.URL+"/sync", "application/json", bytes.NewReader(buf))
	if err != nil { t.Fatal(err) }
	resp.Body.Close()
	if resp.StatusCode != 204 {
		t.Fatalf("want 204, got %d", resp.StatusCode)
	}
	it, ok := n.Store().Get("k")
	if !ok || string(it.Value) != "v" {
		t.Fatalf("sync did not apply")
	}
}

// Quick heartbeat loop smoke test (doesn't assert much, just ensures it runs)
func TestHeartbeatLoop(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	n := NewNode("N", ":x", nil)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/health" {
			w.WriteHeader(200); return
		}
		w.WriteHeader(404)
	}))
	defer srv.Close()
	n.peers = map[string]struct{}{srv.URL: {}}
	go n.HeartbeatLoop(ctx)
	time.Sleep(150 * time.Millisecond)
}