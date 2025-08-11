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
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
)

func fatal(err error) {
		fmt.Fprintln(os.Stderr, "Error:", err)
		os.Exit(1)
}

func main() {
	base := flag.String("server", "http://localhost:8081", "server base URL")
	ttl := flag.String("ttl", "", "TTL for set (e.g. 30s or 60)")
	min := flag.Int("min", 0, "min replication count to wait for")
	full := flag.Bool("full", false, "full replication (wait for all)")
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, `Usage:
  cachectl -server URL get KEY
  cachectl -server URL set KEY VALUE [-ttl=30s] [-min=1] [-full]
  cachectl -server URL del KEY [-min=1] [-full]
`)
		flag.PrintDefaults()
	}
	
	

	flag.Parse()

	if flag.NArg() < 2 {
		flag.Usage()
		os.Exit(2)
	}

	cmd := flag.Arg(0)
	key := flag.Arg(1)

	switch cmd {
	case "get":
		resp, err := http.Get(fmt.Sprintf("%s/kv/%s", *base, key))
		if err != nil { fatal(err) }
		defer resp.Body.Close()
		if resp.StatusCode != 200 {
			io.Copy(os.Stderr, resp.Body)
			os.Exit(1)
		}
		io.Copy(os.Stdout, resp.Body)
	case "set":
		if flag.NArg() < 3 {
			fatal(fmt.Errorf("set requires KEY and VALUE"))
		}
		val := flag.Arg(2)
		url := fmt.Sprintf("%s/kv/%s?min=%d&full=%t", *base, key, *min, *full)
		if *ttl != "" { url += "&ttl=" + *ttl }
		req, _ := http.NewRequest("PUT", url, io.NopCloser(stringsReader(val)))
		resp, err := http.DefaultClient.Do(req)
		if err != nil { fatal(err) }
		defer resp.Body.Close()
		if resp.StatusCode/100 != 2 {
			io.Copy(os.Stderr, resp.Body)
			os.Exit(1)
		}
		fmt.Println("OK")
	case "del":
		url := fmt.Sprintf("%s/kv/%s?min=%d&full=%t", *base, key, *min, *full)
		req, _ := http.NewRequest("DELETE", url, nil)
		resp, err := http.DefaultClient.Do(req)
		if err != nil { fatal(err) }
		defer resp.Body.Close()
		if resp.StatusCode/100 != 2 {
			io.Copy(os.Stderr, resp.Body)
			os.Exit(1)
		}
		fmt.Println("OK")
	default:
		flag.Usage()
		os.Exit(2)
	}
}

func stringsReader(s string) io.ReadCloser { return io.NopCloser(stringsNewReader(s)) }

// tiny local replacements to keep imports minimal
type stringReader struct{ s string; i int64 }
func stringsNewReader(s string) *stringReader { return &stringReader{s: s} }
func (r *stringReader) Read(p []byte) (int, error) {
	if r.i >= int64(len(r.s)) { return 0, io.EOF }
	n := copy(p, r.s[r.i:])
	r.i += int64(n)
	return n, nil
}