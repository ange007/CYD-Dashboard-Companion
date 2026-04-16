package server

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"log"
	"net/http"
	"strings"
	"sync"

	"cyd-companion/internal/collector"
	"cyd-companion/internal/config"
)

// StatusSnapshot is a point-in-time view of companion state for the Web UI.
type StatusSnapshot struct {
	Connected     bool   `json:"connected"`
	DeviceURL     string `json:"device_url"`
	OS            string `json:"os"`
	Hostname      string `json:"hostname"`
	CPU           string `json:"cpu"`
	RAM           string `json:"ram"`
	CPUTemp       string `json:"cpu_temp"`
	ActiveTitle   string `json:"active_title"`
	ActiveProc    string `json:"active_process"`
	ActiveProfile string `json:"active_profile"`
}

// Callbacks allow the server to trigger actions in other parts of the app.
type Callbacks struct {
	GetStatus     func() StatusSnapshot
	GetConfig     func() *config.Config
	SaveConfig    func(*config.Config) error
	GetMetrics    func() []collector.Metric
	GetProfiles   func() ([]map[string]string, string, error)
	SwitchProfile func(id string) error
}

type Server struct {
	port int
	cb   Callbacks
	mu   sync.RWMutex
}

func New(port int, cb Callbacks) *Server {
	return &Server{port: port, cb: cb}
}

// Start registers routes and blocks. Call in a goroutine.
func (s *Server) Start(webFS fs.FS) error {
	mux := http.NewServeMux()

	// REST API — wrap all handlers with CORS so CYD Web UI (different origin) can call /api/metrics
	mux.HandleFunc("/api/status", s.cors(s.handleStatus))
	mux.HandleFunc("/api/config", s.cors(s.handleConfig))
	mux.HandleFunc("/api/metrics", s.cors(s.handleMetrics))
	mux.HandleFunc("/api/profiles", s.cors(s.handleProfiles))
	mux.HandleFunc("/api/profiles/active", s.cors(s.handleProfilesActive))

	// Embedded Web UI
	sub, err := fs.Sub(webFS, "web/dist")
	if err != nil {
		return fmt.Errorf("web FS sub: %w", err)
	}
	fileServer := http.FileServer(http.FS(sub))
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		path := strings.TrimPrefix(r.URL.Path, "/")
		if path == "" {
			path = "index.html"
		}
		f, err := sub.Open(path)
		if err != nil {
			r2 := r.Clone(r.Context())
			r2.URL.Path = "/index.html"
			fileServer.ServeHTTP(w, r2)
			return
		}
		f.Close()
		fileServer.ServeHTTP(w, r)
	})

	addr := fmt.Sprintf("127.0.0.1:%d", s.port)
	log.Printf("[Server] metrics API at http://localhost:%d/api/metrics", s.port)
	return http.ListenAndServe(addr, mux)
}

// cors wraps a handler with permissive CORS headers so the CYD Web UI
// (served from the device at a different origin) can fetch /api/metrics.
func (s *Server) cors(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next(w, r)
	}
}

func (s *Server) handleStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	writeJSON(w, s.cb.GetStatus())
}

func (s *Server) handleMetrics(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	writeJSON(w, s.cb.GetMetrics())
}

func (s *Server) handleConfig(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		writeJSON(w, s.cb.GetConfig())
	case http.MethodPost:
		var cfg config.Config
		if err := json.NewDecoder(r.Body).Decode(&cfg); err != nil {
			http.Error(w, "bad request: "+err.Error(), http.StatusBadRequest)
			return
		}
		if err := s.cb.SaveConfig(&cfg); err != nil {
			http.Error(w, "save failed: "+err.Error(), http.StatusInternalServerError)
			return
		}
		writeJSON(w, map[string]bool{"ok": true})
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func (s *Server) handleProfiles(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	list, activeID, err := s.cb.GetProfiles()
	if err != nil {
		http.Error(w, "device unavailable: "+err.Error(), http.StatusServiceUnavailable)
		return
	}
	writeJSON(w, map[string]interface{}{"list": list, "active_id": activeID})
}

func (s *Server) handleProfilesActive(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var body struct {
		ID string `json:"id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.ID == "" {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	if err := s.cb.SwitchProfile(body.ID); err != nil {
		http.Error(w, "switch failed: "+err.Error(), http.StatusServiceUnavailable)
		return
	}
	writeJSON(w, map[string]bool{"ok": true})
}

func writeJSON(w http.ResponseWriter, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(v)
}
