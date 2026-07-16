// Command burpclone is a minimal MITM proxy / traffic inspector, loosely
// modeled after Burp Suite's Proxy + Repeater workflow.
//
// Build order (see ARCHITECTURE.md for full detail):
//  1. Plain HTTP forward proxy
//  2. HTTPS MITM (CA + dynamic per-host certs)
//  3. SQLite logging (history)
//  4. Interceptor (hold/forward/drop)
//  5. Repeater (edit + refire raw requests)
//  6. Web UI (REST + WebSocket live feed)
package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"strings"

	"burpclone/internal/ca"
	"burpclone/internal/intercept"
	"burpclone/internal/proxy"
	"burpclone/internal/server"
	"burpclone/internal/store"
	"burpclone/internal/reqedit"
)

func main() {
	// --- CLI flags -----------------------------------------------------
	listenAddr := flag.String("listen", ":8080", "address for the proxy to listen on (browser points here)")
	uiAddr := flag.String("ui", ":8000", "address for the web UI/API to listen on")
	caDir := flag.String("ca-dir", "./ca-store", "directory to store/load the root CA cert+key")
	dbPath := flag.String("db", "./burpclone.db", "path to the SQLite history database")
	interceptOn := flag.Bool("intercept", false, "start with interception ON (hold requests for review)")
	excludeHosts := flag.String("exclude", "", "comma-separated wildcard host patterns to skip MITM for, e.g. \"*.bank.co.id,*.some-pinned-app.com\"")
	flag.Parse()

	// --- Phase 2: CA setup ----------------------------------------------
	// TODO(phase 2): ca.LoadOrCreateRootCA should:
	//   - look for root.pem/root.key in caDir
	//   - if missing, generate a new self-signed root CA and save it
	//   - return a *ca.Authority that proxy/tls.go uses to mint leaf certs
	rootCA, err := ca.LoadOrCreateRootCA(*caDir)
	if err != nil {
		log.Fatalf("failed to load/create root CA: %v", err)
	}
	fmt.Println("root CA ready at", *caDir, "- import root.pem into your browser/OS trust store")

	// --- Phase 3: datastore ----------------------------------------------
	// TODO(phase 3): store.Open should open/create the sqlite file and run
	// migrations (create tables if not exist).
	db, err := store.Open(*dbPath)
	if err != nil {
		log.Fatalf("failed to open datastore: %v", err)
	}
	defer db.Close()

	// --- Phase 4: interceptor ---------------------------------------------
	// TODO(phase 4): intercept.NewQueue should expose channels/methods for
	// the UI to list pending requests and call Forward(id)/Drop(id)/Edit(id, ...).
	interceptor := intercept.NewQueue(*interceptOn)

	hub := server.NewHub()

	var excludeList []string
	for _, h := range strings.Split(*excludeHosts, ",") {
		h = strings.TrimSpace(h)
		if h != "" {
			excludeList = append(excludeList, h)
		}
	}

	ruleStore := reqedit.NewRuleStore()
	p := proxy.New(proxy.Options{
		RootCA:       rootCA,
		Store:        db,
		Interceptor:  interceptor,
		Broadcaster:  hub,
		ExcludeHosts: excludeList,
		MatchReplace: ruleStore,
	})

	go func() {
		log.Printf("proxy listening on %s", *listenAddr)
		if err := p.ListenAndServe(*listenAddr); err != nil {
			log.Fatalf("proxy error: %v", err)
		}
	}()

	ui := server.New(server.Options{
		Store:       db,
		Interceptor: interceptor,
		Hub:         hub,
		Rules:       ruleStore,
	})

	log.Printf("web UI listening on %s", *uiAddr)
	if err := ui.ListenAndServe(*uiAddr); err != nil {
		log.Fatal(err)
	}

	os.Exit(0)
}
