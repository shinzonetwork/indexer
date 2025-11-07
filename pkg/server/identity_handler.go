package server

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/shinzonetwork/indexer/pkg/identity"
	"github.com/shinzonetwork/indexer/pkg/logger"
)

// IdentityProvider interface for getting DefraDB identity information
type IdentityProvider interface {
	GetDefraIdentity(ctx context.Context) (*identity.DefraIdentity, error)
	GetDefraPeerID(ctx context.Context) (string, error)
	GetDefraPublicKey(ctx context.Context) (string, error)
	GetDefraAddresses(ctx context.Context) ([]string, error)
}

// IdentityHandler handles HTTP requests for DefraDB identity information
type IdentityHandler struct {
	identityProvider IdentityProvider
}

// NewIdentityHandler creates a new identity handler
func NewIdentityHandler(provider IdentityProvider) *IdentityHandler {
	return &IdentityHandler{
		identityProvider: provider,
	}
}

// HandleIdentity returns the complete DefraDB identity information
func (h *IdentityHandler) HandleIdentity(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	ctx := r.Context()
	identity, err := h.identityProvider.GetDefraIdentity(ctx)
	if err != nil {
		logger.Sugar.Errorf("Failed to get DefraDB identity: %v", err)
		http.Error(w, "Failed to get identity information", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(identity); err != nil {
		logger.Sugar.Errorf("Failed to encode identity response: %v", err)
		http.Error(w, "Failed to encode response", http.StatusInternalServerError)
		return
	}
}

// HandlePeerID returns just the peer ID
func (h *IdentityHandler) HandlePeerID(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	ctx := r.Context()
	peerID, err := h.identityProvider.GetDefraPeerID(ctx)
	if err != nil {
		logger.Sugar.Errorf("Failed to get DefraDB peer ID: %v", err)
		http.Error(w, "Failed to get peer ID", http.StatusInternalServerError)
		return
	}

	response := map[string]string{"peer_id": peerID}
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(response); err != nil {
		logger.Sugar.Errorf("Failed to encode peer ID response: %v", err)
		http.Error(w, "Failed to encode response", http.StatusInternalServerError)
		return
	}
}

// HandlePublicKey returns the public key
func (h *IdentityHandler) HandlePublicKey(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	ctx := r.Context()
	publicKey, err := h.identityProvider.GetDefraPublicKey(ctx)
	if err != nil {
		logger.Sugar.Errorf("Failed to get DefraDB public key: %v", err)
		http.Error(w, "Failed to get public key", http.StatusInternalServerError)
		return
	}

	response := map[string]string{"public_key": publicKey}
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(response); err != nil {
		logger.Sugar.Errorf("Failed to encode public key response: %v", err)
		http.Error(w, "Failed to encode response", http.StatusInternalServerError)
		return
	}
}

// HandleAddresses returns the listening addresses
func (h *IdentityHandler) HandleAddresses(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	ctx := r.Context()
	addresses, err := h.identityProvider.GetDefraAddresses(ctx)
	if err != nil {
		logger.Sugar.Errorf("Failed to get DefraDB addresses: %v", err)
		http.Error(w, "Failed to get addresses", http.StatusInternalServerError)
		return
	}

	response := map[string][]string{"addresses": addresses}
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(response); err != nil {
		logger.Sugar.Errorf("Failed to encode addresses response: %v", err)
		http.Error(w, "Failed to encode response", http.StatusInternalServerError)
		return
	}
}
