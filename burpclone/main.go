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

	"burpclone/internal/ca"
	"burpclone/internal/intercept"
	"burpclone/internal/proxy"
	"burpclone/internal/server"
	"burpclone/internal/store"
)

func main() {
	// --- CLI flags -----------------------------------------------------
	listenAddr := flag.String("listen", ":8080", "address for the proxy to listen on (browser points here)")
	uiAddr := flag.String("ui", ":8000", "address for the web UI/API to listen on")
	caDir := flag.String("ca-dir", "./ca-store", "directory to store/load the root CA cert+key")
	dbPath := flag.String("db", "./burpclone.db", "path to the SQLite history database")
	interceptOn := flag.Bool("intercept", false, "start with interception ON (hold requests for review)")
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

	// --- Phase 1+2: proxy engine ------------------------------------------
	// TODO(phase 1): proxy.New should set up a plain HTTP forward proxy.
	// TODO(phase 2): extend it to handle CONNECT + MITM using rootCA.
	p := proxy.New(proxy.Options{
		RootCA:      rootCA,
		Store:       db,
		Interceptor: interceptor,
	})

	go func() {
		log.Printf("proxy listening on %s", *listenAddr)
		if err := p.ListenAndServe(*listenAddr); err != nil {
			log.Fatalf("proxy error: %v", err)
		}
	}()

	// --- Phase 6: web UI (REST + WebSocket) --------------------------------
	// TODO(phase 6): server.New should wire up:
	//   - REST endpoints to browse history from the store
	//   - REST endpoints to control the interceptor (list/forward/drop/edit)
	//   - REST endpoint for the Repeater (resend an edited raw request)
	//   - WebSocket endpoint streaming new traffic as it happens
	ui := server.New(server.Options{
		Store:       db,
		Interceptor: interceptor,
	})

	log.Printf("web UI listening on %s", *uiAddr)
	if err := ui.ListenAndServe(*uiAddr); err != nil {
		log.Fatal(err)
	}

	os.Exit(0)
}
