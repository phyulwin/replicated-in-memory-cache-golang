// Author: Phyu Lwin
// Project: Replicated In-Memory Cache (Golang)
// Date: Aug 10th, 2025
//
// Summary:
// This file defines the HTTP API endpoints for the replicated in-memory cache node.
// It provides handlers for health checks, key-value operations (GET, PUT, DELETE),
// and synchronization between nodes. The endpoints support replication controls
// and TTL (time-to-live) for cache entries. The file also includes utility functions
// for parsing request paths, durations, and managing replication acknowledgments.

package cache

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"
)

func (n *Node) Routes() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(200)
		w.Write([]byte("ok"))
	})
	mux.HandleFunc("GET /kv/", n.handleGet)
	mux.HandleFunc("PUT /kv/", n.handlePut)
	mux.HandleFunc("DELETE /kv/", n.handleDelete)
	mux.HandleFunc("POST /sync", n.handleSync)
	return logging(mux)
}

func keyFromPath(path string) (string, error) {
	parts := strings.SplitN(strings.TrimPrefix(path, "/kv/"), "/", 2)
	if len(parts) == 0 || parts[0] == "" {
		return "", errors.New("missing key")
	}
	return parts[0], nil
}

func (n *Node) handleGet(w http.ResponseWriter, r *http.Request) {
	key, err := keyFromPath(r.URL.Path)
	if err != nil {
		http.Error(w, err.Error(), 400); return
	}
	it, ok := n.store.Get(key)
	now := time.Now()
	if !ok || it.Tombstone || it.expired(now) {
		http.NotFound(w, r); return
	}
	w.Header().Set("Content-Type", "application/octet-stream")
	w.WriteHeader(200)
	w.Write(it.Value)
}

func parseDurationQS(v string) (time.Duration, error) {
	if v == "" {
		return 0, nil
	}
	if d, err := time.ParseDuration(v); err == nil {
		return d, nil
	}
	secs, err := strconv.ParseInt(v, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("bad ttl: %w", err)
	}
	return time.Duration(secs) * time.Second, nil
}

func (n *Node) handlePut(w http.ResponseWriter, r *http.Request) {
	key, err := keyFromPath(r.URL.Path)
	if err != nil { http.Error(w, err.Error(), 400); return }
	body, err := io.ReadAll(r.Body)
	if err != nil { http.Error(w, "read body error", 400); return }

	ttl, err := parseDurationQS(r.URL.Query().Get("ttl"))
	if err != nil { http.Error(w, err.Error(), 400); return }

	minRep := 0
	if q := r.URL.Query().Get("min"); q != "" {
		if v, err := strconv.Atoi(q); err == nil && v >= 0 { minRep = v }
	}
	full := r.URL.Query().Get("full") == "true"

	version := time.Now().UnixNano()
	item := Item{
		Value:   body,
		Version: version,
		Origin:  n.ID,
	}
	if ttl > 0 {
		item.ExpiresAt = time.Now().Add(ttl)
	}

	applied := n.store.Put(key, item)
	if !applied {
		http.Error(w, "write lost to newer version", 409)
		return
	}

	acked, total, err := n.Replicate(r.Context(), SyncMsg{
		Op:        "set",
		Key:       key,
		Value:     body,
		ExpiresAt: ptrTimeOrNil(item.ExpiresAt),
		Version:   version,
		Origin:    n.ID,
	}, minRep, full)

	if err != nil {
		http.Error(w, fmt.Sprintf("replication error: %v (acked %d/%d)", err, acked, total), 502)
		return
	}

	w.Header().Set("X-Replicated-Acked", fmt.Sprintf("%d", acked))
	w.Header().Set("X-Replicated-Total", fmt.Sprintf("%d", total))
	w.WriteHeader(201)
}

func (n *Node) handleDelete(w http.ResponseWriter, r *http.Request) {
	key, err := keyFromPath(r.URL.Path)
	if err != nil { http.Error(w, err.Error(), 400); return }

	minRep := 0
	if q := r.URL.Query().Get("min"); q != "" {
		if v, err := strconv.Atoi(q); err == nil && v >= 0 { minRep = v }
	}
	full := r.URL.Query().Get("full") == "true"

	version := time.Now().UnixNano()
	it := Item{Version: version, Origin: n.ID, Tombstone: true}
	n.store.Put(key, it)

	acked, total, err := n.Replicate(r.Context(), SyncMsg{
		Op:      "del",
		Key:     key,
		Version: version,
		Origin:  n.ID,
	}, minRep, full)

	if err != nil {
		http.Error(w, fmt.Sprintf("replication error: %v (acked %d/%d)", err, acked, total), 502)
		return
	}
	w.Header().Set("X-Replicated-Acked", fmt.Sprintf("%d", acked))
	w.Header().Set("X-Replicated-Total", fmt.Sprintf("%d", total))
	w.WriteHeader(204)
}

func (n *Node) handleSync(w http.ResponseWriter, r *http.Request) {
	var msg SyncMsg
	if err := json.NewDecoder(r.Body).Decode(&msg); err != nil {
		http.Error(w, "bad json", 400); return
	}
	switch msg.Op {
	case "set":
		item := Item{Value: msg.Value, Version: msg.Version, Origin: msg.Origin}
		if msg.ExpiresAt != nil { item.ExpiresAt = *msg.ExpiresAt }
		n.store.Put(msg.Key, item)
	case "del":
		n.store.Put(msg.Key, Item{Version: msg.Version, Origin: msg.Origin, Tombstone: true})
	default:
		http.Error(w, "unknown op", 400); return
	}
	w.WriteHeader(204)
}