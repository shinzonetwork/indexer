package auth

import (
	"crypto/subtle"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/shinzonetwork/indexer/pkg/logger"
)

// KeyringAuthRequest represents the authentication request
type KeyringAuthRequest struct {
	KeyringSecret string `json:"keyring_secret"`
}

// KeyringAuthenticator handles keyring-based authentication
type KeyringAuthenticator struct {
	expectedSecret string
}

// NewKeyringAuthenticator creates a new keyring authenticator
func NewKeyringAuthenticator(secret string) *KeyringAuthenticator {
	return &KeyringAuthenticator{
		expectedSecret: secret,
	}
}

// ValidateKeyringSecret validates the provided keyring secret
func (a *KeyringAuthenticator) ValidateKeyringSecret(providedSecret string) bool {
	if a.expectedSecret == "" {
		logger.Sugar.Warn("No keyring secret configured - authentication disabled")
		return false
	}
	
	// Use constant-time comparison to prevent timing attacks
	return subtle.ConstantTimeCompare([]byte(providedSecret), []byte(a.expectedSecret)) == 1
}

// AuthenticateRequest validates the keyring secret from HTTP request
func (a *KeyringAuthenticator) AuthenticateRequest(r *http.Request) (bool, error) {
	// Check for JSON body with keyring_secret
	if r.Header.Get("Content-Type") == "application/json" {
		var authReq KeyringAuthRequest
		if err := json.NewDecoder(r.Body).Decode(&authReq); err != nil {
			return false, fmt.Errorf("invalid JSON body: %w", err)
		}
		return a.ValidateKeyringSecret(authReq.KeyringSecret), nil
	}
	
	// Check for Authorization header
	authHeader := r.Header.Get("Authorization")
	if authHeader != "" {
		// Support "Bearer <keyring_secret>" format
		if strings.HasPrefix(authHeader, "Bearer ") {
			secret := strings.TrimPrefix(authHeader, "Bearer ")
			return a.ValidateKeyringSecret(secret), nil
		}
		// Support "Keyring <keyring_secret>" format
		if strings.HasPrefix(authHeader, "Keyring ") {
			secret := strings.TrimPrefix(authHeader, "Keyring ")
			return a.ValidateKeyringSecret(secret), nil
		}
	}
	
	// Check for X-Keyring-Secret header
	keyringHeader := r.Header.Get("X-Keyring-Secret")
	if keyringHeader != "" {
		return a.ValidateKeyringSecret(keyringHeader), nil
	}
	
	return false, fmt.Errorf("no keyring secret provided")
}

// RequireKeyringAuth is a middleware that requires keyring authentication
func (a *KeyringAuthenticator) RequireKeyringAuth(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		authenticated, err := a.AuthenticateRequest(r)
		if err != nil {
			logger.Sugar.Warnf("Authentication error: %v", err)
			http.Error(w, "Authentication required", http.StatusUnauthorized)
			return
		}
		
		if !authenticated {
			logger.Sugar.Warn("Invalid keyring secret provided")
			http.Error(w, "Invalid keyring secret", http.StatusUnauthorized)
			return
		}
		
		// Authentication successful, proceed to handler
		next(w, r)
	}
}
