/*
Author:      Phyu Lwin
Project:     Replicated In-Memory Cache (Golang)
Date:        Aug 10th 2025

Summary:
This file defines core types used internally by the replicated cache module.
It includes the Item struct representing a cache entry with metadata for expiration,
versioning, and tombstone (deletion) marking, as well as the SyncMsg struct for
synchronization messages between nodes.

Functions in this file:
- (Item) expired(now time.Time) bool
*/

package cache

import "time"

// Public-ish types used across files (kept internal to the module).

type Item struct {
	Value     []byte    `json:"value,omitempty"`
	ExpiresAt time.Time `json:"expires_at,omitempty"`
	Version   int64     `json:"version"`   // ns since epoch (origin’s clock)
	Origin    string    `json:"origin"`    // node id
	Tombstone bool      `json:"tombstone"` // deletion marker
}

func (it Item) expired(now time.Time) bool {
	return !it.ExpiresAt.IsZero() && now.After(it.ExpiresAt)
}

type SyncMsg struct {
	Op        string     `json:"op"` // "set" or "del"
	Key       string     `json:"key"`
	Value     []byte     `json:"value,omitempty"`
	ExpiresAt *time.Time `json:"expires_at,omitempty"`
	Version   int64      `json:"version"`
	Origin    string     `json:"origin"`
}