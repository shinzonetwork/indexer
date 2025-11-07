package defra

import (
	"strconv"
	"strings"

	"github.com/sourcenetwork/defradb/node"
)

func GetPort(targetNode *node.Node) int {
	url := targetNode.APIURL
	return GetPortFromUrl(url)
}

func GetPortFromUrl(url string) int {
	// Extract port from URL - supports any IP address format (localhost, 127.0.0.1, or LAN IP)
	// URL format: http://IP:PORT or http://IP:PORT/path
	
	// Find the port by looking for the pattern after the last colon before the first slash
	// First, remove the protocol if present
	if strings.HasPrefix(url, "http://") {
		url = strings.TrimPrefix(url, "http://")
	} else if strings.HasPrefix(url, "https://") {
		url = strings.TrimPrefix(url, "https://")
	}
	
	// Split by colon - the last part before any slash should be the port
	parts := strings.Split(url, ":")
	if len(parts) < 2 {
		return -1
	}

	// Get the last part (port) and remove any trailing path
	portStr := parts[len(parts)-1]
	if strings.Contains(portStr, "/") {
		portStr = strings.Split(portStr, "/")[0]
	}

	// Convert port string to int
	portInt, err := strconv.Atoi(portStr)
	if err != nil {
		return -1
	}

	return portInt
}
