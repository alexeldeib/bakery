package main

import (
	"context"
	"encoding/json"
	"io/ioutil"
	"net/http"
	"net/http/pprof"
	"time"

	"github.com/go-logr/logr"
	"github.com/gorilla/mux"
)

type HTTPServer struct {
	log    logr.Logger
	bakery *Bakery
	srv    http.Server
}

func (h *HTTPServer) Start() error {
	return h.srv.ListenAndServe()
}

func (h *HTTPServer) Shutdown(ctx context.Context) {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	h.srv.Shutdown(ctx)
}

func (h *HTTPServer) putSnapshot(w http.ResponseWriter, r *http.Request) {
	// Read
	defer r.Body.Close()
	body, err := ioutil.ReadAll(r.Body)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}

	// Parse
	var request snapshot
	if err = json.Unmarshal(body, &request); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	// Validate
	if err := request.isValid(); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	// Execute
	if err := h.bakery.createSnapshot(&request); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Return
	handleOK(w, r)
}

// func (h *HTTPServer) getSnapshot(w http.ResponseWriter, r *http.Request) {
// 	handleOK(w, r)
// }

// func (h *HTTPServer) deleteSnapshot(w http.ResponseWriter, r *http.Request) {
// 	handleOK(w, r)
// }

func newHTTPServer(addr string, bakery *Bakery, log logr.Logger) *HTTPServer {
	s := &HTTPServer{
		log:    log,
		bakery: bakery,
	}

	router := mux.NewRouter()

	// Consider disabling
	router.Handle("/debug/pprof/", http.HandlerFunc(pprof.Index))
	router.Handle("/debug/pprof/cmdline", http.HandlerFunc(pprof.Cmdline))
	router.Handle("/debug/pprof/profile", http.HandlerFunc(pprof.Profile))
	router.Handle("/debug/pprof/symbol", http.HandlerFunc(pprof.Symbol))
	router.Handle("/debug/pprof/trace", http.HandlerFunc(pprof.Trace))

	// logic
	router.Handle("/snapshot", http.HandlerFunc(s.putSnapshot)).Methods("PUT")
	// router.Handle("/snapshot", http.HandlerFunc(s.getSnapshot)).Methods("GET")
	// router.Handle("/snapshot", http.HandlerFunc(s.deleteSnapshot)).Methods("DELETE")

	s.srv = http.Server{
		Addr:         addr,
		ReadTimeout:  3600 * time.Second, // TODO(ace): split into frontend/async with workqueue?
		WriteTimeout: 3600 * time.Second,
		IdleTimeout:  3600 * time.Second,
		Handler:      router,
	}

	return s
}

// func healthz(w http.ResponseWriter, r *http.Request) {
// 	handleOK(w, r)
// }

// func livez(w http.ResponseWriter, r *http.Request) {
// 	handleOK(w, r)
// }

// func readyz(w http.ResponseWriter, r *http.Request) {
// 	handleOK(w, r)
// }

func setHeaders(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.Header().Set("X-Content-Type-Options", "nosniff")
}

func handleOK(w http.ResponseWriter, r *http.Request) {
	setHeaders(w)
	w.WriteHeader(http.StatusOK)
}
