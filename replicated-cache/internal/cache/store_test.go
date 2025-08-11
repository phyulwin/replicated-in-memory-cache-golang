/*
Author: Phyu Lwin
Project: Replicated In-Memory Cache Golang
Date: Aug 10th, 2025

Summary:
	This file contains unit tests for the Store implementation in the replicated in-memory cache project.
	The tests verify the correctness of Last-Write-Wins (LWW) semantics, including versioning and origin-based tie-breaking,
	as well as the handling of TTL (time-to-live) expiration and tombstone garbage collection.

List of functions:
	- TestStoreLWW: Tests LWW semantics, including version comparison and origin-based tie-breaking.
	- TestStoreTTLAndTombstoneGC: Tests TTL expiration and garbage collection of tombstone entries.
*/

package cache

import (
	"testing"
	"time"
)

func TestStoreLWW(t *testing.T) {
	s := NewStore()
	// First write
	ok := s.Put("k", Item{Value: []byte("a"), Version: 1, Origin: "A"})
	if !ok { t.Fatal("first put should apply") }
	// Older write should NOT win
	ok = s.Put("k", Item{Value: []byte("b"), Version: 0, Origin: "B"})
	if ok { t.Fatal("older write should not apply") }
	// Same version, lexicographically larger Origin wins
	ok = s.Put("k", Item{Value: []byte("c"), Version: 1, Origin: "Z"})
	if !ok { t.Fatal("tie-break should apply") }
	got, _ := s.Get("k")
	if string(got.Value) != "c" {
		t.Fatalf("want c, got %q", string(got.Value))
	}
}

func TestStoreTTLAndTombstoneGC(t *testing.T) {
	s := NewStore()
	now := time.Now()
	s.Put("ttl", Item{Value: []byte("v"), Version: 1, ExpiresAt: now.Add(10 * time.Millisecond)})
	s.Put("del", Item{Tombstone: true, Version: now.Add(-10 * time.Minute).UnixNano()})
	time.Sleep(20 * time.Millisecond)
	s.HardDeleteExpired(time.Now(), 1*time.Minute) // tombstone ttl not reached yet
	if _, ok := s.Get("ttl"); ok {
		t.Fatal("ttl entry should be removed")
	}
	// Now GC tombstone by using small TTL
	s.HardDeleteExpired(time.Now(), 1*time.Nanosecond)
	if _, ok := s.Get("del"); ok {
		t.Fatal("tombstone should be removed")
	}
}