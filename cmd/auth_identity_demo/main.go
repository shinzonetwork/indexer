package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/shinzonetwork/indexer/pkg/auth"
	"github.com/shinzonetwork/indexer/pkg/identity"
	"github.com/shinzonetwork/indexer/pkg/logger"
	"github.com/shinzonetwork/indexer/pkg/server"

	defrahttp "github.com/sourcenetwork/defradb/http"
	"github.com/sourcenetwork/defradb/node"
)

const testKeyringSecret = "test-keyring-secret-123"

func main() {
	fmt.Println("🔐 Authenticated DefraDB Identity Demo")
	fmt.Println("=====================================")

	// Initialize logger
	logger.Init(true)

	// Create a temporary DefraDB node for demonstration
	ctx := context.Background()
	
	// Configure DefraDB node
	defrahttp.PlaygroundEnabled = true
	
	options := []node.Option{
		node.WithDisableAPI(false),
		node.WithDisableP2P(true), // Disable P2P for demo
		node.WithStorePath("./.defra-auth-demo"),
		defrahttp.WithAddress("127.0.0.1:9184"), // Use different port to avoid conflicts
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

	// Create identity provider
	identityProvider := identity.NewDefraNodeIdentityProvider(defraNode)

	// Start authenticated HTTP server
	fmt.Println("\n🌐 Starting Authenticated HTTP API Server on :8082")
	fmt.Println("=================================================")
	
	mux := http.NewServeMux()
	
	// Create authenticated identity handler
	authHandler := server.NewAuthenticatedIdentityHandler(identityProvider, testKeyringSecret)
	
	// Register authenticated endpoints
	mux.HandleFunc("/auth/node-identity", authHandler.HandleAuthenticatedNodeIdentity)
	mux.HandleFunc("/auth/public-key", authHandler.HandleAuthenticatedPublicKey)
	mux.HandleFunc("/auth/peer-id", authHandler.HandleAuthenticatedPeerID)
	
	// Also register public endpoints for comparison
	publicHandler := server.NewIdentityHandler(identityProvider)
	mux.HandleFunc("/public/identity", publicHandler.HandleIdentity)
	
	// Root handler with instructions
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, `Authenticated DefraDB Identity API Demo

🔐 AUTHENTICATED ENDPOINTS (require keyring_secret):
- POST /auth/node-identity  - Complete identity (requires auth)
- POST /auth/public-key     - Public key only (requires auth)  
- POST /auth/peer-id        - Peer ID only (requires auth)

🌐 PUBLIC ENDPOINTS (no auth required):
- GET /public/identity      - Complete identity (public)

Authentication Methods:
1. JSON Body: {"keyring_secret": "%s"}
2. Authorization Header: "Bearer %s"
3. Authorization Header: "Keyring %s"  
4. X-Keyring-Secret Header: "%s"

Examples:
curl -X POST http://localhost:8082/auth/public-key \
  -H "Content-Type: application/json" \
  -d '{"keyring_secret": "%s"}'

curl -X POST http://localhost:8082/auth/public-key \
  -H "Authorization: Bearer %s"

curl -X POST http://localhost:8082/auth/public-key \
  -H "X-Keyring-Secret: %s"
`, testKeyringSecret, testKeyringSecret, testKeyringSecret, testKeyringSecret, testKeyringSecret, testKeyringSecret, testKeyringSecret)
	})

	server := &http.Server{
		Addr:    ":8082",
		Handler: mux,
	}

	// Start server in background
	go func() {
		log.Fatal(server.ListenAndServe())
	}()

	// Wait for server to start
	time.Sleep(1 * time.Second)

	fmt.Printf("🔑 Test Keyring Secret: %s\n", testKeyringSecret)
	fmt.Println("\n📡 Available Endpoints:")
	fmt.Println("  POST http://localhost:8082/auth/node-identity  (authenticated)")
	fmt.Println("  POST http://localhost:8082/auth/public-key     (authenticated)")
	fmt.Println("  POST http://localhost:8082/auth/peer-id        (authenticated)")
	fmt.Println("  GET  http://localhost:8082/public/identity     (public)")

	// Demonstrate the API calls
	fmt.Println("\n🧪 Testing API Calls:")
	fmt.Println("====================")

	// Test 1: Public endpoint (no auth)
	fmt.Println("\n1. Testing public endpoint (no auth required):")
	testPublicEndpoint()

	// Test 2: Authenticated endpoint with correct secret
	fmt.Println("\n2. Testing authenticated endpoint with correct secret:")
	testAuthenticatedEndpoint("/auth/public-key", testKeyringSecret, true)

	// Test 3: Authenticated endpoint with wrong secret
	fmt.Println("\n3. Testing authenticated endpoint with wrong secret:")
	testAuthenticatedEndpoint("/auth/public-key", "wrong-secret", false)

	// Test 4: Authenticated endpoint with no secret
	fmt.Println("\n4. Testing authenticated endpoint with no secret:")
	testAuthenticatedEndpointNoAuth("/auth/public-key")

	// Test 5: Different authentication methods
	fmt.Println("\n5. Testing different authentication methods:")
	testAuthenticationMethods()

	fmt.Println("\n⏹️  Demo completed. Press Ctrl+C to stop server")
	
	// Keep server running
	select {}
}

func testPublicEndpoint() {
	resp, err := http.Get("http://localhost:8082/public/identity")
	if err != nil {
		fmt.Printf("❌ Error: %v\n", err)
		return
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	fmt.Printf("✅ Status: %d\n", resp.StatusCode)
	fmt.Printf("📄 Response: %s\n", string(body))
}

func testAuthenticatedEndpoint(endpoint, secret string, shouldSucceed bool) {
	payload := map[string]string{"keyring_secret": secret}
	jsonData, _ := json.Marshal(payload)

	resp, err := http.Post("http://localhost:8082"+endpoint, "application/json", bytes.NewBuffer(jsonData))
	if err != nil {
		fmt.Printf("❌ Error: %v\n", err)
		return
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	
	if shouldSucceed && resp.StatusCode == 200 {
		fmt.Printf("✅ Status: %d (Expected success)\n", resp.StatusCode)
		fmt.Printf("📄 Response: %s\n", string(body))
	} else if !shouldSucceed && resp.StatusCode == 401 {
		fmt.Printf("✅ Status: %d (Expected failure)\n", resp.StatusCode)
		fmt.Printf("📄 Response: %s\n", string(body))
	} else {
		fmt.Printf("❌ Unexpected status: %d\n", resp.StatusCode)
		fmt.Printf("📄 Response: %s\n", string(body))
	}
}

func testAuthenticatedEndpointNoAuth(endpoint string) {
	resp, err := http.Post("http://localhost:8082"+endpoint, "application/json", bytes.NewBuffer([]byte("{}")))
	if err != nil {
		fmt.Printf("❌ Error: %v\n", err)
		return
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	
	if resp.StatusCode == 401 {
		fmt.Printf("✅ Status: %d (Expected failure - no auth)\n", resp.StatusCode)
		fmt.Printf("📄 Response: %s\n", string(body))
	} else {
		fmt.Printf("❌ Unexpected status: %d\n", resp.StatusCode)
		fmt.Printf("📄 Response: %s\n", string(body))
	}
}

func testAuthenticationMethods() {
	endpoint := "http://localhost:8082/auth/public-key"
	
	// Test Bearer token
	fmt.Println("\n  a) Bearer token authentication:")
	req, _ := http.NewRequest("POST", endpoint, bytes.NewBuffer([]byte("{}")))
	req.Header.Set("Authorization", "Bearer "+testKeyringSecret)
	req.Header.Set("Content-Type", "application/json")
	
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		fmt.Printf("❌ Error: %v\n", err)
	} else {
		defer resp.Body.Close()
		body, _ := io.ReadAll(resp.Body)
		fmt.Printf("✅ Status: %d\n", resp.StatusCode)
		fmt.Printf("📄 Response: %s\n", string(body))
	}

	// Test X-Keyring-Secret header
	fmt.Println("\n  b) X-Keyring-Secret header authentication:")
	req, _ = http.NewRequest("POST", endpoint, bytes.NewBuffer([]byte("{}")))
	req.Header.Set("X-Keyring-Secret", testKeyringSecret)
	req.Header.Set("Content-Type", "application/json")
	
	resp, err = client.Do(req)
	if err != nil {
		fmt.Printf("❌ Error: %v\n", err)
	} else {
		defer resp.Body.Close()
		body, _ := io.ReadAll(resp.Body)
		fmt.Printf("✅ Status: %d\n", resp.StatusCode)
		fmt.Printf("📄 Response: %s\n", string(body))
	}

	// Test Keyring header
	fmt.Println("\n  c) Keyring header authentication:")
	req, _ = http.NewRequest("POST", endpoint, bytes.NewBuffer([]byte("{}")))
	req.Header.Set("Authorization", "Keyring "+testKeyringSecret)
	req.Header.Set("Content-Type", "application/json")
	
	resp, err = client.Do(req)
	if err != nil {
		fmt.Printf("❌ Error: %v\n", err)
	} else {
		defer resp.Body.Close()
		body, _ := io.ReadAll(resp.Body)
		fmt.Printf("✅ Status: %d\n", resp.StatusCode)
		fmt.Printf("📄 Response: %s\n", string(body))
	}
}
