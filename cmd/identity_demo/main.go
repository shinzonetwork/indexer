package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/shinzonetwork/indexer/pkg/identity"
	"github.com/shinzonetwork/indexer/pkg/logger"

	defrahttp "github.com/sourcenetwork/defradb/http"
	"github.com/sourcenetwork/defradb/node"
)

func main() {
	fmt.Println("🔑 DefraDB Identity Demo")
	fmt.Println("========================")

	// Initialize logger
	logger.Init(true)

	// Create a temporary DefraDB node for demonstration
	ctx := context.Background()
	
	// Configure DefraDB node
	defrahttp.PlaygroundEnabled = true
	
	options := []node.Option{
		node.WithDisableAPI(false),
		node.WithDisableP2P(false), // Enable P2P to get peer identity
		node.WithStorePath("./.defra-demo"),
		defrahttp.WithAddress("127.0.0.1:9182"), // Use different port to avoid conflicts
	}

	fmt.Println("🚀 Starting DefraDB node...")
	defraNode, err := node.New(ctx, options...)
	if err != nil {
		log.Fatalf("Failed to create DefraDB node: %v", err)
	}

	err = defraNode.Start(ctx)
	if err != nil {
		log.Fatalf("Failed to start DefraDB node: %v", err)
	}
	defer defraNode.Close(ctx)

	fmt.Println("✅ DefraDB node started successfully!")
	
	// Wait a moment for P2P system to fully initialize
	fmt.Println("⏳ Waiting for P2P system to initialize...")
	time.Sleep(2 * time.Second)

	// Create identity provider
	identityProvider := identity.NewDefraNodeIdentityProvider(defraNode)

	// Check if peer is initialized
	if defraNode.Peer == nil {
		log.Fatalf("P2P peer is not initialized - this is required for identity access")
	}

	// Demonstrate identity retrieval
	fmt.Println("\n🔍 Retrieving DefraDB Identity Information:")
	fmt.Println("==========================================")

	// Get complete identity
	identity, err := identityProvider.GetIdentity(ctx)
	if err != nil {
		log.Fatalf("Failed to get identity: %v", err)
	}

	// Pretty print the identity
	identityJSON, _ := json.MarshalIndent(identity, "", "  ")
	fmt.Printf("Complete Identity:\n%s\n", identityJSON)

	// Get individual components
	fmt.Println("\n📋 Individual Components:")
	fmt.Println("========================")

	peerID, err := identityProvider.GetPeerID(ctx)
	if err != nil {
		log.Printf("Failed to get peer ID: %v", err)
	} else {
		fmt.Printf("Peer ID: %s\n", peerID)
	}

	publicKey, err := identityProvider.GetPublicKey(ctx)
	if err != nil {
		log.Printf("Failed to get public key: %v", err)
	} else {
		fmt.Printf("Public Key: %s\n", publicKey)
	}

	addresses, err := identityProvider.GetAddresses(ctx)
	if err != nil {
		log.Printf("Failed to get addresses: %v", err)
	} else {
		fmt.Printf("Addresses: %v\n", addresses)
	}

	// Start a simple HTTP server to demonstrate the API
	fmt.Println("\n🌐 Starting HTTP API Demo Server on :8081")
	fmt.Println("=========================================")
	
	mux := http.NewServeMux()
	
	// Note: This demo shows the identity interface, but in production
	// these endpoints are now authenticated and require keyring_secret
	// See cmd/auth_identity_demo for authenticated examples
	
	// Create identity handler for demo purposes
	identityHandler := &IdentityHandler{identityProvider: identityProvider}
	
	mux.HandleFunc("/demo/identity", identityHandler.HandleIdentity)
	mux.HandleFunc("/demo/peer-id", identityHandler.HandlePeerID)
	mux.HandleFunc("/demo/public-key", identityHandler.HandlePublicKey)
	mux.HandleFunc("/demo/addresses", identityHandler.HandleAddresses)
	
	// Root handler with instructions
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, `DefraDB Identity API Demo (Unauthenticated)

⚠️  IMPORTANT: In production, identity endpoints require authentication!
    See cmd/auth_identity_demo for authenticated examples.

Demo endpoints (for testing only):
- GET /demo/identity          - Complete identity information
- GET /demo/peer-id          - Just the peer ID
- GET /demo/public-key       - Just the public key
- GET /demo/addresses        - Just the addresses

Production endpoints (require keyring_secret):
- POST /auth/node-identity   - Complete identity (authenticated)
- POST /auth/public-key      - Public key (authenticated)
- POST /auth/peer-id         - Peer ID (authenticated)

Example:
curl http://localhost:8081/demo/identity
`)
	})

	server := &http.Server{
		Addr:    ":8081",
		Handler: mux,
	}

	fmt.Println("📡 Demo API Endpoints:")
	fmt.Println("  GET http://localhost:8081/demo/identity")
	fmt.Println("  GET http://localhost:8081/demo/peer-id")
	fmt.Println("  GET http://localhost:8081/demo/public-key")
	fmt.Println("  GET http://localhost:8081/demo/addresses")
	fmt.Println("\n💡 Try: curl http://localhost:8081/demo/identity")
	fmt.Println("\n⚠️  Note: Production endpoints require authentication!")
	fmt.Println("\n⏹️  Press Ctrl+C to stop")

	// Start server
	log.Fatal(server.ListenAndServe())
}

// IdentityHandler demonstrates the HTTP interface
type IdentityHandler struct {
	identityProvider identity.DefraIdentityProvider
}

func (h *IdentityHandler) HandleIdentity(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	identity, err := h.identityProvider.GetIdentity(ctx)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to get identity: %v", err), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(identity)
}

func (h *IdentityHandler) HandlePeerID(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	peerID, err := h.identityProvider.GetPeerID(ctx)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to get peer ID: %v", err), http.StatusInternalServerError)
		return
	}

	response := map[string]string{"peer_id": peerID}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

func (h *IdentityHandler) HandlePublicKey(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	publicKey, err := h.identityProvider.GetPublicKey(ctx)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to get public key: %v", err), http.StatusInternalServerError)
		return
	}

	response := map[string]string{"public_key": publicKey}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

func (h *IdentityHandler) HandleAddresses(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	addresses, err := h.identityProvider.GetAddresses(ctx)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to get addresses: %v", err), http.StatusInternalServerError)
		return
	}

	response := map[string][]string{"addresses": addresses}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}
