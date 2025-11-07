package identity

import (
	"context"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"

	"github.com/sourcenetwork/defradb/node"
)

// DefraIdentity represents the cryptographic identity of a DefraDB node
type DefraIdentity struct {
	PeerID      string   `json:"peer_id"`
	PublicKey   string   `json:"public_key"`
	Addresses   []string `json:"addresses"`
	DID         string   `json:"did,omitempty"`
}

// DefraIdentityProvider interface for accessing DefraDB identity information
type DefraIdentityProvider interface {
	// GetIdentity returns the complete identity information for the DefraDB node
	GetIdentity(ctx context.Context) (*DefraIdentity, error)
	
	// GetPeerID returns just the peer ID
	GetPeerID(ctx context.Context) (string, error)
	
	// GetPublicKey returns the public key in hex format
	GetPublicKey(ctx context.Context) (string, error)
	
	// GetAddresses returns the listening addresses
	GetAddresses(ctx context.Context) ([]string, error)
	
	// GetKeyInfo returns information about key persistence and reuse
	GetKeyInfo(ctx context.Context, storePath string) (*KeyInfo, error)
}

// DefraNodeIdentityProvider implements DefraIdentityProvider for embedded DefraDB nodes
type DefraNodeIdentityProvider struct {
	node *node.Node
}

// NewDefraNodeIdentityProvider creates a new identity provider for an embedded DefraDB node
func NewDefraNodeIdentityProvider(defraNode *node.Node) DefraIdentityProvider {
	return &DefraNodeIdentityProvider{
		node: defraNode,
	}
}

// GetIdentity returns the complete identity information for the DefraDB node
func (p *DefraNodeIdentityProvider) GetIdentity(ctx context.Context) (*DefraIdentity, error) {
	if p.node == nil || p.node.DB == nil {
		return nil, fmt.Errorf("DefraDB node or database is not initialized")
	}

	// Get the P2P peer information (libp2p peer ID and addresses)
	peerInfo := p.node.DB.PeerInfo()
	
	// Get the ACP node identity (public key and DID)
	nodeIdentityOpt, err := p.node.DB.GetNodeIdentity(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get node identity: %w", err)
	}

	var publicKeyHex, did string
	
	// Check if ACP identity is available
	if nodeIdentityOpt.HasValue() {
		nodeIdentity := nodeIdentityOpt.Value()
		publicKeyHex = nodeIdentity.PublicKey // Already a hex string
		did = nodeIdentity.DID
	} else {
		// If ACP identity is not available, we can still provide peer info
		publicKeyHex = "" // Will be empty if ACP is not configured
		did = ""
	}
	
	return &DefraIdentity{
		PeerID:    peerInfo.ID,        // Use actual libp2p peer ID
		PublicKey: publicKeyHex,       // ACP public key (may be empty)
		Addresses: peerInfo.Addresses, // libp2p addresses
		DID:       did,               // ACP DID (may be empty)
	}, nil
}

// GetPeerID returns just the peer ID
func (p *DefraNodeIdentityProvider) GetPeerID(ctx context.Context) (string, error) {
	identity, err := p.GetIdentity(ctx)
	if err != nil {
		return "", err
	}
	return identity.PeerID, nil
}

// GetPublicKey returns the public key in hex format
func (p *DefraNodeIdentityProvider) GetPublicKey(ctx context.Context) (string, error) {
	identity, err := p.GetIdentity(ctx)
	if err != nil {
		return "", err
	}
	return identity.PublicKey, nil
}

// GetAddresses returns the network addresses
func (p *DefraNodeIdentityProvider) GetAddresses(ctx context.Context) ([]string, error) {
	identity, err := p.GetIdentity(ctx)
	if err != nil {
		return nil, err
	}
	return identity.Addresses, nil
}

// CheckKeyExists checks if a persistent key already exists in the DefraDB store
// DefraDB automatically reuses the same key pair if it exists in the store path
func (p *DefraNodeIdentityProvider) CheckKeyExists(storePath string) (bool, string, error) {
	// Check multiple possible key file locations
	keyPaths := []string{
		filepath.Join(storePath, "keys", "peer_id"),
		filepath.Join(storePath, "keys", "swarm.key"),
		filepath.Join(storePath, "peer_id"),
		filepath.Join(storePath, "swarm.key"),
		filepath.Join(storePath, "config", "peer_id"),
	}
	
	for _, keyPath := range keyPaths {
		if _, err := os.Stat(keyPath); err == nil {
			// Key file exists - with new DefraDB API, we can't directly access peer info
			// but we know a key exists
			return true, "", nil
		}
	}
	
	return false, "", nil
}

// GetKeyInfo returns information about the current key
// Note: Key persistence is now handled by DefraDB app-sdk
func (p *DefraNodeIdentityProvider) GetKeyInfo(ctx context.Context, storePath string) (*KeyInfo, error) {
	var currentPeerID, currentPublicKey string
	
	// Get current identity if node is running
	if p.node != nil && p.node.DB != nil {
		identity, err := p.GetIdentity(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to get current identity: %v", err)
		}
		currentPeerID = identity.PeerID
		currentPublicKey = identity.PublicKey
	}
	
	return &KeyInfo{
		KeyExists:        currentPeerID != "", // Key exists if we have a current peer ID
		ExistingPeerID:   currentPeerID,       // Current peer ID is the "existing" one
		CurrentPeerID:    currentPeerID,
		CurrentPublicKey: currentPublicKey,
		StorePath:        storePath,
		IsReused:         true, // Assume DefraDB app-sdk handles persistence
	}, nil
}

// KeyInfo contains information about the DefraDB key state
type KeyInfo struct {
	KeyExists        bool   `json:"key_exists"`
	ExistingPeerID   string `json:"existing_peer_id,omitempty"`
	CurrentPeerID    string `json:"current_peer_id,omitempty"`
	CurrentPublicKey string `json:"current_public_key,omitempty"`
	StorePath        string `json:"store_path"`
	IsReused         bool   `json:"is_reused"`
}


// generateDID attempts to generate a DID from the public key (optional feature)
func (p *DefraNodeIdentityProvider) generateDID(publicKeyBytes []byte) (string, error) {
	// This is a simplified DID generation - you might want to use a proper DID library
	// For now, we'll create a basic did:key format
	if len(publicKeyBytes) == 0 {
		return "", fmt.Errorf("empty public key")
	}
	
	// Basic did:key format (this is simplified - real implementation would use proper multicodec)
	return fmt.Sprintf("did:key:z%s", hex.EncodeToString(publicKeyBytes)[:32]), nil
}


// HTTPIdentityProvider implements DefraIdentityProvider for external DefraDB nodes via HTTP
type HTTPIdentityProvider struct {
	baseURL string
}

// NewHTTPIdentityProvider creates a new identity provider for external DefraDB nodes
func NewHTTPIdentityProvider(baseURL string) DefraIdentityProvider {
	return &HTTPIdentityProvider{
		baseURL: baseURL,
	}
}

// GetIdentity returns identity information via HTTP API calls
func (p *HTTPIdentityProvider) GetIdentity(ctx context.Context) (*DefraIdentity, error) {
	// This would make HTTP calls to DefraDB's API endpoints
	// Implementation depends on what endpoints DefraDB exposes
	return nil, fmt.Errorf("HTTP identity provider not yet implemented - use embedded node")
}

// GetPeerID returns peer ID via HTTP
func (p *HTTPIdentityProvider) GetPeerID(ctx context.Context) (string, error) {
	return "", fmt.Errorf("HTTP identity provider not yet implemented - use embedded node")
}

// GetPublicKey returns public key via HTTP
func (p *HTTPIdentityProvider) GetPublicKey(ctx context.Context) (string, error) {
	return "", fmt.Errorf("HTTP identity provider not yet implemented - use embedded node")
}

// GetAddresses returns addresses via HTTP
func (p *HTTPIdentityProvider) GetAddresses(ctx context.Context) ([]string, error) {
	return nil, fmt.Errorf("HTTP identity provider not yet implemented - use embedded node")
}

// GetKeyInfo returns key information via HTTP
func (p *HTTPIdentityProvider) GetKeyInfo(ctx context.Context, storePath string) (*KeyInfo, error) {
	return nil, fmt.Errorf("HTTP identity provider not yet implemented - use embedded node")
}
