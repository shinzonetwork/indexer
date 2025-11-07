package server

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/shinzonetwork/indexer/pkg/auth"
	"github.com/shinzonetwork/indexer/pkg/identity"
	"github.com/shinzonetwork/indexer/pkg/logger"
)

// AuthenticatedIdentityHandler handles authenticated requests for DefraDB identity information
type AuthenticatedIdentityHandler struct {
	identityProvider IdentityProvider
	authenticator    *auth.KeyringAuthenticator
}

// NewAuthenticatedIdentityHandler creates a new authenticated identity handler
func NewAuthenticatedIdentityHandler(provider IdentityProvider, keyringSecret string) *AuthenticatedIdentityHandler {
	return &AuthenticatedIdentityHandler{
		identityProvider: provider,
		authenticator:    auth.NewKeyringAuthenticator(keyringSecret),
	}
}

// HandleAuthenticatedNodeIdentity returns the DefraDB node identity with keyring authentication
func (h *AuthenticatedIdentityHandler) HandleAuthenticatedNodeIdentity(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed - use POST with keyring_secret", http.StatusMethodNotAllowed)
		return
	}

	// Authenticate the request
	authenticated, err := h.authenticator.AuthenticateRequest(r)
	if err != nil {
		logger.Sugar.Warnf("Authentication error: %v", err)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnauthorized)
		json.NewEncoder(w).Encode(map[string]string{
			"error": "Authentication required",
			"message": "Provide keyring_secret in JSON body, Authorization header, or X-Keyring-Secret header",
		})
		return
	}

	if !authenticated {
		logger.Sugar.Warn("Invalid keyring secret provided for node identity access")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnauthorized)
		json.NewEncoder(w).Encode(map[string]string{
			"error": "Invalid keyring secret",
			"message": "The provided keyring secret is incorrect",
		})
		return
	}

	// Authentication successful - get the identity
	ctx := r.Context()
	identity, err := h.identityProvider.GetDefraIdentity(ctx)
	if err != nil {
		logger.Sugar.Errorf("Failed to get DefraDB identity: %v", err)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{
			"error": "Failed to get identity information",
		})
		return
	}

	logger.Sugar.Infof("Node identity accessed successfully")
	
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(identity); err != nil {
		logger.Sugar.Errorf("Failed to encode identity response: %v", err)
		http.Error(w, "Failed to encode response", http.StatusInternalServerError)
		return
	}
}

// HandleAuthenticatedPublicKey returns just the public key with authentication
func (h *AuthenticatedIdentityHandler) HandleAuthenticatedPublicKey(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed - use POST with keyring_secret", http.StatusMethodNotAllowed)
		return
	}

	// Authenticate the request
	authenticated, err := h.authenticator.AuthenticateRequest(r)
	if err != nil {
		logger.Sugar.Warnf("Authentication error: %v", err)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnauthorized)
		json.NewEncoder(w).Encode(map[string]string{
			"error": "Authentication required",
			"message": "Provide keyring_secret in JSON body, Authorization header, or X-Keyring-Secret header",
		})
		return
	}

	if !authenticated {
		logger.Sugar.Warn("Invalid keyring secret provided for public key access")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnauthorized)
		json.NewEncoder(w).Encode(map[string]string{
			"error": "Invalid keyring secret",
			"message": "The provided keyring secret is incorrect",
		})
		return
	}

	// Authentication successful - get the public key
	ctx := r.Context()
	publicKey, err := h.identityProvider.GetDefraPublicKey(ctx)
	if err != nil {
		logger.Sugar.Errorf("Failed to get DefraDB public key: %v", err)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{
			"error": "Failed to get public key",
		})
		return
	}

	logger.Sugar.Infof("Public key accessed successfully")

	response := map[string]string{"public_key": publicKey}
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(response); err != nil {
		logger.Sugar.Errorf("Failed to encode public key response: %v", err)
		http.Error(w, "Failed to encode response", http.StatusInternalServerError)
		return
	}
}

// HandleAuthenticatedPeerID handles requests for just the peer ID (authenticated)
func (h *AuthenticatedIdentityHandler) HandleAuthenticatedPeerID(w http.ResponseWriter, r *http.Request) {
	// Only allow POST requests
	if r.Method != http.MethodPost {
		http.Error(w, `{"error":"Method not allowed - use POST with keyring_secret"}`, http.StatusMethodNotAllowed)
		return
	}

	// Authenticate the request
	authenticated, err := h.authenticator.AuthenticateRequest(r)
	if err != nil {
		logger.Sugar.Warnf("Authentication error: %v", err)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnauthorized)
		json.NewEncoder(w).Encode(map[string]string{
			"error": "Authentication required",
			"message": "Provide keyring_secret in JSON body, Authorization header, or X-Keyring-Secret header",
		})
		return
	}

	if !authenticated {
		logger.Sugar.Warn("Invalid keyring secret provided for peer ID access")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnauthorized)
		json.NewEncoder(w).Encode(map[string]string{
			"error": "Invalid keyring secret",
			"message": "The provided keyring secret is incorrect",
		})
		return
	}

	// Authentication successful - get the peer ID
	ctx := r.Context()
	peerID, err := h.identityProvider.GetDefraPeerID(ctx)
	if err != nil {
		logger.Sugar.Errorf("Failed to get DefraDB peer ID: %v", err)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{
			"error": "Failed to get peer ID",
		})
		return
	}

	logger.Sugar.Infof("Peer ID accessed successfully")

	response := map[string]string{"peer_id": peerID}
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(response); err != nil {
		logger.Sugar.Errorf("Failed to encode peer ID response: %v", err)
		http.Error(w, "Failed to encode response", http.StatusInternalServerError)
		return
	}
}

// HandleAuthenticatedKeyInfo handles requests for key information (authenticated)
func (h *AuthenticatedIdentityHandler) HandleAuthenticatedKeyInfo(w http.ResponseWriter, r *http.Request) {
	// Only allow POST requests
	if r.Method != http.MethodPost {
		http.Error(w, `{"error":"Method not allowed - use POST with keyring_secret"}`, http.StatusMethodNotAllowed)
		return
	}

	// Authenticate the request
	authenticated, err := h.authenticator.AuthenticateRequest(r)
	if err != nil {
		logger.Sugar.Warnf("Authentication error: %v", err)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnauthorized)
		json.NewEncoder(w).Encode(map[string]string{
			"error": "Authentication required",
			"message": "Provide keyring_secret in JSON body, Authorization header, or X-Keyring-Secret header",
		})
		return
	}

	if !authenticated {
		logger.Sugar.Warn("Invalid keyring secret provided for key info access")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnauthorized)
		json.NewEncoder(w).Encode(map[string]string{
			"error": "Invalid keyring secret",
			"message": "The provided keyring secret is incorrect",
		})
		return
	}

	// Check if we have an indexer with key info capability
	if indexer, ok := h.identityProvider.(interface {
		GetKeyInfo(ctx context.Context) (*identity.KeyInfo, error)
	}); ok {
		ctx := r.Context()
		keyInfo, err := indexer.GetKeyInfo(ctx)
		if err != nil {
			logger.Sugar.Errorf("Failed to get key info: %v", err)
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(map[string]string{
				"error": "Failed to get key info",
			})
			return
		}

		logger.Sugar.Infof("Key info accessed successfully")
		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(keyInfo); err != nil {
			logger.Sugar.Errorf("Failed to encode key info response: %v", err)
			http.Error(w, "Failed to encode response", http.StatusInternalServerError)
			return
		}
	} else {
		logger.Sugar.Warn("Key info not available - identity provider doesn't support GetKeyInfo")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotImplemented)
		json.NewEncoder(w).Encode(map[string]string{
			"error": "Key info not available",
			"message": "This identity provider doesn't support key information",
		})
	}
}

