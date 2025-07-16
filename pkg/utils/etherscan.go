package utils

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/ethereum/go-ethereum/accounts/abi"
)

// EtherscanResponse represents the response from Etherscan API
type EtherscanResponse struct {
	Status  string `json:"status"`
	Message string `json:"message"`
	Result  string `json:"result"`
}

// ABICache stores ABIs in memory to avoid repeated API calls
type ABICache struct {
	mu    sync.RWMutex
	cache map[string]*abi.ABI
}

var (
	abiCache = &ABICache{
		cache: make(map[string]*abi.ABI),
	}
	etherscanAPIKey = getEtherscanAPIKey()
)

// getEtherscanAPIKey gets API key from environment or returns default
func getEtherscanAPIKey() string {
	if key := os.Getenv("ETHERSCAN_API_KEY"); key != "" {
		return key
	}
	// Fallback to your provided key (consider moving to env var for security)
	return "4IQER572ZNIJ16SHJE5FMGD1HNIMF8ACZM"
}

// GetContractABI fetches contract ABI from Etherscan API
func GetContractABI(address string) (string, error) {
	// Clean the address
	address = strings.ToLower(strings.TrimSpace(address))
	if !strings.HasPrefix(address, "0x") {
		return "", fmt.Errorf("invalid address format: %s", address)
	}

	client := &http.Client{
		Timeout: 10 * time.Second,
	}

	url := fmt.Sprintf("https://api.etherscan.io/api?module=contract&action=getabi&address=%s&apikey=%s", 
		address, etherscanAPIKey)

	resp, err := client.Get(url)
	if err != nil {
		return "", fmt.Errorf("failed to fetch ABI from Etherscan: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("Etherscan API returned status %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read response body: %w", err)
	}

	var etherscanResp EtherscanResponse
	if err := json.Unmarshal(body, &etherscanResp); err != nil {
		return "", fmt.Errorf("failed to parse Etherscan response: %w", err)
	}

	if etherscanResp.Status != "1" {
		return "", fmt.Errorf("Etherscan API error: %s", etherscanResp.Message)
	}

	return etherscanResp.Result, nil
}

// GetOrFetchABI gets ABI from cache or fetches from Etherscan
func GetOrFetchABI(address string) (*abi.ABI, error) {
	address = strings.ToLower(address)
	
	// Check cache first
	abiCache.mu.RLock()
	if cachedABI, exists := abiCache.cache[address]; exists {
		abiCache.mu.RUnlock()
		return cachedABI, nil
	}
	abiCache.mu.RUnlock()

	// Fetch from Etherscan
	abiJSON, err := GetContractABI(address)
	if err != nil {
		return nil, err
	}

	// Parse ABI
	parsedABI, err := abi.JSON(strings.NewReader(abiJSON))
	if err != nil {
		return nil, fmt.Errorf("failed to parse ABI JSON: %w", err)
	}

	// Cache the ABI
	abiCache.mu.Lock()
	abiCache.cache[address] = &parsedABI
	abiCache.mu.Unlock()

	return &parsedABI, nil
}

// LoadContractABIs loads ABIs for multiple contract addresses
func LoadContractABIs(addresses []string) (map[string]*abi.ABI, error) {
	abis := make(map[string]*abi.ABI)
	errors := make([]string, 0)

	for _, addr := range addresses {
		if parsedABI, err := GetOrFetchABI(addr); err != nil {
			errors = append(errors, fmt.Sprintf("%s: %v", addr, err))
		} else {
			abis[strings.ToLower(addr)] = parsedABI
		}
		
		// Add a small delay to avoid hitting rate limits
		time.Sleep(200 * time.Millisecond)
	}

	if len(errors) > 0 {
		return abis, fmt.Errorf("failed to load some ABIs: %s", strings.Join(errors, "; "))
	}

	return abis, nil
}

// ClearABICache clears the ABI cache
func ClearABICache() {
	abiCache.mu.Lock()
	defer abiCache.mu.Unlock()
	abiCache.cache = make(map[string]*abi.ABI)
}
