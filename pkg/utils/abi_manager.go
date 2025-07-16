package utils

import (
	"strings"
	"sync"

	"github.com/ethereum/go-ethereum/accounts/abi"
	gethtypes "github.com/ethereum/go-ethereum/core/types"
)

// ABIManager manages contract ABIs for the blockchain indexer
type ABIManager struct {
	mu               sync.RWMutex
	contractABIs     map[string]*abi.ABI
	knownContracts   []string
	autoFetch        bool
}

// NewABIManager creates a new ABI manager
func NewABIManager(autoFetch bool) *ABIManager {
	return &ABIManager{
		contractABIs:   make(map[string]*abi.ABI),
		knownContracts: make([]string, 0),
		autoFetch:      autoFetch,
	}
}

// LoadABIForAddress loads ABI for a specific contract address
func (am *ABIManager) LoadABIForAddress(address string) error {
	address = strings.ToLower(address)
	
	am.mu.Lock()
	defer am.mu.Unlock()
	
	// Check if already loaded
	if _, exists := am.contractABIs[address]; exists {
		return nil
	}
	
	// Fetch ABI
	parsedABI, err := GetOrFetchABI(address)
	if err != nil {
		return err
	}
	
	am.contractABIs[address] = parsedABI
	am.knownContracts = append(am.knownContracts, address)
	
	return nil
}

// GetABIs returns a copy of the current ABI map for use in conversion
func (am *ABIManager) GetABIs() map[string]*abi.ABI {
	am.mu.RLock()
	defer am.mu.RUnlock()
	
	// Create a copy to avoid race conditions
	abis := make(map[string]*abi.ABI)
	for addr, abi := range am.contractABIs {
		abis[addr] = abi
	}
	
	return abis
}

// ProcessLogsForNewContracts scans logs to find new contract addresses and optionally fetches their ABIs
func (am *ABIManager) ProcessLogsForNewContracts(logs []*gethtypes.Log) {
	if !am.autoFetch {
		return
	}
	
	newAddresses := make(map[string]bool)
	
	// Collect unique contract addresses from logs
	for _, log := range logs {
		addr := strings.ToLower(log.Address.Hex())
		
		am.mu.RLock()
		_, exists := am.contractABIs[addr]
		am.mu.RUnlock()
		
		if !exists && !newAddresses[addr] {
			newAddresses[addr] = true
		}
	}
	
	// Fetch ABIs for new addresses
	for addr := range newAddresses {
		if err := am.LoadABIForAddress(addr); err != nil {
			// Log error but don't fail the entire process
			// You might want to use your logger here
			continue
		}
	}
}

// GetKnownContracts returns the list of known contract addresses
func (am *ABIManager) GetKnownContracts() []string {
	am.mu.RLock()
	defer am.mu.RUnlock()
	
	contracts := make([]string, len(am.knownContracts))
	copy(contracts, am.knownContracts)
	
	return contracts
}

// LoadKnownContracts preloads ABIs for a list of known important contracts
func (am *ABIManager) LoadKnownContracts(addresses []string) error {
	for _, addr := range addresses {
		if err := am.LoadABIForAddress(addr); err != nil {
			return err
		}
	}
	return nil
}
