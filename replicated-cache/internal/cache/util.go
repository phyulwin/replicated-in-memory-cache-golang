/*
Author: phyu lwin
Project: replicated-in-memory-cache-golang
Date: Aug 10th 2025

Summary:
This file provides utility functions and middleware for the replicated in-memory cache project.
It includes a helper for safely returning a pointer to a time.Time value, as well as an HTTP middleware
for logging request details and response status codes. Additionally, it defines a custom response recorder
to capture HTTP status codes for logging purposes.

List of functions:
- ptrTimeOrNil(t time.Time) *time.Time
- logging(next http.Handler) http.Handler
- (rr *respRecorder) WriteHeader(code int)
*/

package cache

import (
	"log"
	"net/http"
	"time"
)

func ptrTimeOrNil(t time.Time) *time.Time {
	if t.IsZero() { return nil }
	return &t
}

func logging(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		rr := &respRecorder{ResponseWriter: w, status: 200}
		next.ServeHTTP(rr, r)
		d := time.Since(start)
		log.Printf("%s %s -> %d (%s)", r.Method, r.URL.Path, rr.status, d)
	})
}

type respRecorder struct {
	http.ResponseWriter
	status int
}
func (rr *respRecorder) WriteHeader(code int) { rr.status = code; rr.ResponseWriter.WriteHeader(code) }