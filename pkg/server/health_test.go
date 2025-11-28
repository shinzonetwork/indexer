package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

type mockHealthChecker struct {
	healthy       bool
	currentBlock  int64
	lastProcessed time.Time
	p2pInfo       *P2PInfo
	p2pErr        error
	defraReg      DefraPKRegistration
	peerReg       PeerIDRegistration
	signErr       error
}

func (m *mockHealthChecker) IsHealthy() bool { return m.healthy }

func (m *mockHealthChecker) GetCurrentBlock() int64 { return m.currentBlock }

func (m *mockHealthChecker) GetLastProcessedTime() time.Time { return m.lastProcessed }

func (m *mockHealthChecker) GetPeerInfo() (*P2PInfo, error) { return m.p2pInfo, m.p2pErr }

func (m *mockHealthChecker) SignMessages(message string) (DefraPKRegistration, PeerIDRegistration, error) {
	return m.defraReg, m.peerReg, m.signErr
}

func TestHealthHandler_IncludesRegistrationOnSuccess(t *testing.T) {
	mock := &mockHealthChecker{
		healthy:       true,
		currentBlock:  42,
		lastProcessed: time.Now(),
		p2pInfo: &P2PInfo{
			Enabled:  true,
			PeerInfo: []PeerInfo{{ID: "peer1"}},
		},
		defraReg: DefraPKRegistration{
			PublicKey:   "0xpubkey",
			SignedPKMsg: "0xsigned-pk",
		},
		peerReg: PeerIDRegistration{
			PeerID:        "0xpeer1",
			SignedPeerMsg: "0xsigned-peer",
		},
	}

	hs := NewHealthServer(0, mock, "")
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/registration", nil)

	hs.registrationHandler(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)

	var resp HealthResponse
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))

	require.NotNil(t, resp.Registration)
	require.True(t, resp.Registration.Enabled)
	require.Equal(t, "Shinzo Network Indexer registration", resp.Registration.Message)
	require.Equal(t, "0xpubkey", resp.Registration.DefraPKRegistration.PublicKey)
	require.Equal(t, "0xsigned-pk", resp.Registration.DefraPKRegistration.SignedPKMsg)
	require.Equal(t, "0xpeer1", resp.Registration.PeerIDRegistration.PeerID)
	require.Equal(t, "0xsigned-peer", resp.Registration.PeerIDRegistration.SignedPeerMsg)
}

func TestHealthHandler_RegistrationDisabledOnSignError(t *testing.T) {
	mock := &mockHealthChecker{
		healthy:       true,
		currentBlock:  1,
		lastProcessed: time.Now(),
		p2pInfo: &P2PInfo{
			Enabled:  true,
			PeerInfo: []PeerInfo{{ID: "peer1"}},
		},
		signErr: errTestSignFailed{},
	}

	hs := NewHealthServer(0, mock, "")
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/registration", nil)

	hs.registrationHandler(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)

	var resp HealthResponse
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))

	require.NotNil(t, resp.Registration)
	require.False(t, resp.Registration.Enabled)
}

type errTestSignFailed struct{}

func (errTestSignFailed) Error() string { return "sign failed" }
