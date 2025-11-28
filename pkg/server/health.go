package server

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/shinzonetwork/indexer/pkg/logger"
)

// HealthServer provides HTTP endpoints for health checks and metrics
type HealthServer struct {
	server   *http.Server
	indexer  HealthChecker
	defraURL string
}

// HealthChecker interface for checking indexer health
type HealthChecker interface {
	IsHealthy() bool
	GetCurrentBlock() int64
	GetLastProcessedTime() time.Time
	GetPeerInfo() (*P2PInfo, error)
	SignMessages(message string) (DefraPKRegistration, PeerIDRegistration, error)
}

// P2PInfo represents DefraDB P2P network information
type P2PInfo struct {
	Enabled  bool       `json:"enabled"`
	PeerInfo []PeerInfo `json:"peers"`
}

type PeerInfo struct {
	ID        string   `json:"id"`
	Addresses []string `json:"addresses"`
	PublicKey string   `json:"public_key,omitempty"`
}

type DisplayRegistration struct {
	Enabled             bool                `json:"enabled"`
	Message             string              `json:"message"`
	DefraPKRegistration DefraPKRegistration `json:"defra_pk_registration,omitempty"`
	PeerIDRegistration  PeerIDRegistration  `json:"peer_id_registration,omitempty"`
}

type DefraPKRegistration struct {
	PublicKey   string `json:"public_key,omitempty"`
	SignedPKMsg string `json:"signed_pk_message,omitempty"`
}

type PeerIDRegistration struct {
	PeerID        string `json:"peer_id,omitempty"`
	SignedPeerMsg string `json:"signed_peer_message,omitempty"`
}

// HealthResponse represents the health check response
type HealthResponse struct {
	Status           string               `json:"status"`
	Timestamp        time.Time            `json:"timestamp"`
	CurrentBlock     int64                `json:"current_block,omitempty"`
	LastProcessed    time.Time            `json:"last_processed,omitempty"`
	DefraDBConnected bool                 `json:"defradb_connected"`
	Uptime           string               `json:"uptime"`
	P2P              *P2PInfo             `json:"p2p,omitempty"`
	Registration     *DisplayRegistration `json:"registration,omitempty"`
}

// MetricsResponse represents basic metrics
type MetricsResponse struct {
	BlocksProcessed   int64     `json:"blocks_processed"`
	CurrentBlock      int64     `json:"current_block"`
	LastProcessedTime time.Time `json:"last_processed_time"`
	Uptime            string    `json:"uptime"`
}

var startTime = time.Now()

// NewHealthServer creates a new health server
func NewHealthServer(port int, indexer HealthChecker, defraURL string) *HealthServer {
	mux := http.NewServeMux()

	hs := &HealthServer{
		server: &http.Server{
			Addr:         fmt.Sprintf(":%d", port),
			Handler:      mux,
			ReadTimeout:  10 * time.Second,
			WriteTimeout: 10 * time.Second,
		},
		indexer:  indexer,
		defraURL: defraURL,
	}

	// Register routes
	mux.HandleFunc("/health", hs.healthHandler)
	mux.HandleFunc("/registration", hs.registrationHandler)
	mux.HandleFunc("/metrics", hs.metricsHandler)
	mux.HandleFunc("/", hs.rootHandler)

	return hs
}

// Start starts the health server
func (hs *HealthServer) Start() error {
	logger.Sugar.Infof("Starting health server on %s", hs.server.Addr)
	return hs.server.ListenAndServe()
}

// Stop gracefully stops the health server
func (hs *HealthServer) Stop(ctx context.Context) error {
	logger.Sugar.Info("Stopping health server...")
	return hs.server.Shutdown(ctx)
}

// healthHandler handles liveness probe requests
func (hs *HealthServer) healthHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	response := HealthResponse{
		Status:           "healthy",
		Timestamp:        time.Now(),
		DefraDBConnected: hs.checkDefraDB(),
		Uptime:           time.Since(startTime).String(),
	}

	if hs.indexer != nil {
		response.CurrentBlock = hs.indexer.GetCurrentBlock()
		response.LastProcessed = hs.indexer.GetLastProcessedTime()
		p2p, err := hs.indexer.GetPeerInfo()
		response.P2P = p2p
		if err != nil {
			response.Status = "unhealthy"
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}

		if !hs.indexer.IsHealthy() {
			response.Status = "unhealthy"
			w.WriteHeader(http.StatusServiceUnavailable)
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// registrationHandler handles readiness probe requests
func (hs *HealthServer) registrationHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Check if indexer is ready (has processed at least one block recently)
	ready := true
	if hs.indexer != nil {
		lastProcessed := hs.indexer.GetLastProcessedTime()
		if time.Since(lastProcessed) > 5*time.Minute && !lastProcessed.IsZero() {
			ready = false
		}
	}

	// Check DefraDB connectivity
	if !hs.checkDefraDB() {
		ready = false
	}

	response := HealthResponse{
		Status:           "ready",
		Timestamp:        time.Now(),
		DefraDBConnected: hs.checkDefraDB(),
		Uptime:           time.Since(startTime).String(),
	}

	if hs.indexer != nil {
		response.CurrentBlock = hs.indexer.GetCurrentBlock()
		response.LastProcessed = hs.indexer.GetLastProcessedTime()
		p2p, err := hs.indexer.GetPeerInfo()
		response.P2P = p2p
		if err != nil {
			response.Status = "unhealthy"
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}

		// Include signed registration information on the registration endpoint.
		const registrationMessage = "Shinzo Network Indexer registration"
		defraReg, peerReg, signErr := hs.indexer.SignMessages(registrationMessage)
		registration := &DisplayRegistration{
			Enabled: signErr == nil,
			Message: registrationMessage,
		}
		if signErr == nil {
			// Normalize signed fields to 0x-prefixed hex strings for API consumers.
			defraReg.PublicKey = normalizeHex(defraReg.PublicKey)
			peerReg.PeerID = normalizeHex(peerReg.PeerID)
			defraReg.SignedPKMsg = normalizeHex(defraReg.SignedPKMsg)
			peerReg.SignedPeerMsg = normalizeHex(peerReg.SignedPeerMsg)
			registration.DefraPKRegistration = defraReg
			registration.PeerIDRegistration = peerReg
		}
		response.Registration = registration
	}

	if !ready {
		response.Status = "not ready"
		w.WriteHeader(http.StatusServiceUnavailable)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// metricsHandler provides basic metrics in JSON format
func (hs *HealthServer) metricsHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	metrics := MetricsResponse{
		Uptime: time.Since(startTime).String(),
	}

	if hs.indexer != nil {
		metrics.CurrentBlock = hs.indexer.GetCurrentBlock()
		metrics.LastProcessedTime = hs.indexer.GetLastProcessedTime()
		metrics.BlocksProcessed = hs.indexer.GetCurrentBlock() // Simplified
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(metrics)
}

// rootHandler handles root requests
func (hs *HealthServer) rootHandler(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}

	response := map[string]interface{}{
		"service":   "Shinzo Network Indexer",
		"version":   "1.0.0",
		"status":    "running",
		"timestamp": time.Now(),
		"endpoints": []string{
			"/health 	   - Health probe",
			"/registration - Registration information",
			"/metrics 	   - Basic metrics",
		},
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// checkDefraDB checks if DefraDB is accessible
func (hs *HealthServer) checkDefraDB() bool {
	if hs.defraURL == "" {
		return true // Embedded mode, assume healthy
	}

	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Get(hs.defraURL + "/api/v0/graphql")
	if err != nil {
		return false
	}
	defer resp.Body.Close()

	return resp.StatusCode == http.StatusOK || resp.StatusCode == http.StatusBadRequest // GraphQL endpoint returns 400 for GET
}

// normalizeHex ensures a string is represented as a 0x-prefixed hex string.
// If the string is empty, it is returned unchanged.
func normalizeHex(s string) string {
	if s == "" {
		return s
	}
	if strings.HasPrefix(s, "0x") || strings.HasPrefix(s, "0X") {
		// Normalize any 0X to 0x for consistency.
		return "0x" + s[2:]
	}
	return "0x" + s
}
