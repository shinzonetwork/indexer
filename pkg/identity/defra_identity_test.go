package identity

import (
	"context"
	"testing"

	"github.com/shinzonetwork/indexer/pkg/logger"
	defrahttp "github.com/sourcenetwork/defradb/http"
	"github.com/sourcenetwork/defradb/node"
)

func TestDefraNodeIdentityProvider(t *testing.T) {
	// Initialize logger for tests
	logger.Init(true)

	ctx := context.Background()

	// Create a test DefraDB node
	defrahttp.PlaygroundEnabled = false
	options := []node.Option{
		node.WithDisableAPI(false),
		node.WithDisableP2P(true),
		node.WithStorePath("./.defra-test"),
		defrahttp.WithAddress("127.0.0.1:9183"), // Use different port for tests
	}

	defraNode, err := node.New(ctx, options...)
	if err != nil {
		t.Fatalf("Failed to create DefraDB node: %v", err)
	}

	err = defraNode.Start(ctx)
	if err != nil {
		t.Fatalf("Failed to start DefraDB node: %v", err)
	}
	defer defraNode.Close(ctx)

	// Create identity provider
	provider := NewDefraNodeIdentityProvider(defraNode)

	// Test GetPeerID
	t.Run("GetPeerID", func(t *testing.T) {
		peerID, err := provider.GetPeerID(ctx)
		if err != nil {
			t.Errorf("GetPeerID failed: %v", err)
		}
		if peerID == "" {
			t.Error("PeerID should not be empty")
		}
		t.Logf("PeerID: %s", peerID)
	})

	// Test GetPublicKey
	t.Run("GetPublicKey", func(t *testing.T) {
		publicKey, err := provider.GetPublicKey(ctx)
		if err != nil {
			t.Errorf("GetPublicKey failed: %v", err)
		}
		if publicKey == "" {
			t.Error("PublicKey should not be empty")
		}
		t.Logf("PublicKey: %s", publicKey)
	})

	// Test GetAddresses
	t.Run("GetAddresses", func(t *testing.T) {
		addresses, err := provider.GetAddresses(ctx)
		if err != nil {
			t.Errorf("GetAddresses failed: %v", err)
		}
		if len(addresses) == 0 {
			t.Error("Addresses should not be empty")
		}
		t.Logf("Addresses: %v", addresses)
	})

	// Test GetIdentity
	t.Run("GetIdentity", func(t *testing.T) {
		identity, err := provider.GetIdentity(ctx)
		if err != nil {
			t.Errorf("GetIdentity failed: %v", err)
		}
		if identity == nil {
			t.Error("Identity should not be nil")
		}
		if identity.PeerID == "" {
			t.Error("Identity PeerID should not be empty")
		}
		if identity.PublicKey == "" {
			t.Error("Identity PublicKey should not be empty")
		}
		if len(identity.Addresses) == 0 {
			t.Error("Identity Addresses should not be empty")
		}
		t.Logf("Complete Identity: %+v", identity)
	})
}

func TestHTTPIdentityProvider(t *testing.T) {
	// Test that HTTP provider returns appropriate errors for unimplemented methods
	provider := NewHTTPIdentityProvider("http://localhost:9181")
	ctx := context.Background()

	t.Run("GetIdentity_NotImplemented", func(t *testing.T) {
		_, err := provider.GetIdentity(ctx)
		if err == nil {
			t.Error("Expected error for unimplemented HTTP provider")
		}
	})

	t.Run("GetPeerID_NotImplemented", func(t *testing.T) {
		_, err := provider.GetPeerID(ctx)
		if err == nil {
			t.Error("Expected error for unimplemented HTTP provider")
		}
	})
}
