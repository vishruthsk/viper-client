package rpc

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"sync"
	"time"

	"github.com/illegalcall/viper-client/internal/models"
)

var (
	// ErrNoEndpoints is returned when no active endpoints are available for a chain
	ErrNoEndpoints = errors.New("no active endpoints available for the requested chain")
)

// Dispatcher handles forwarding RPC requests to blockchain nodes
type Dispatcher struct {
	endpointManager     EndpointManager
	httpClient          *http.Client
	viperNetworkHandler *ViperNetworkHandler
	lock                sync.RWMutex
	// Cache could be added here for common requests
}

// EndpointManager defines the interface for retrieving and managing RPC endpoints
type EndpointManager interface {
	GetActiveEndpoints(chainID int) ([]models.RpcEndpoint, error)
	UpdateEndpointHealth(id int, status string) error
}

// NewDispatcher creates a new RPC dispatcher with the given endpoint manager
func NewDispatcher(manager EndpointManager) *Dispatcher {
	viperHandler := NewViperNetworkHandler(manager)

	return &Dispatcher{
		endpointManager: manager,
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
		viperNetworkHandler: viperHandler,
	}
}

// RPCRequest represents a JSON-RPC request
type RPCRequest struct {
	JSONRPC string      `json:"jsonrpc"`
	Method  string      `json:"method"`
	Params  interface{} `json:"params"`
	ID      interface{} `json:"id"`
}

// RPCResponse represents a JSON-RPC response
type RPCResponse struct {
	JSONRPC string      `json:"jsonrpc"`
	Result  interface{} `json:"result,omitempty"`
	Error   interface{} `json:"error,omitempty"`
	ID      interface{} `json:"id"`
}

// Forward forwards an RPC request to an available endpoint for the given chain
func (d *Dispatcher) Forward(ctx context.Context, chainID int, requestBody []byte) ([]byte, error) {
	// Check if this is a request for the Viper Network
	if chainID == ViperNetworkChainID {
		return d.ForwardToViperNetwork(ctx, requestBody)
	}

	// Parse the incoming request to validate and potentially use for caching
	var rpcRequest RPCRequest
	if err := json.Unmarshal(requestBody, &rpcRequest); err != nil {
		return nil, errors.New("invalid JSON-RPC request format")
	}

	// Get available endpoints for the chain
	endpoints, err := d.endpointManager.GetActiveEndpoints(chainID)
	if err != nil {
		return nil, err
	}

	if len(endpoints) == 0 {
		return nil, ErrNoEndpoints
	}

	// Sort endpoints by priority (assuming they are already sorted from the database)
	// For now, just use the first endpoint
	selectedEndpoint := endpoints[0]

	// Forward the request to the selected endpoint
	req, err := http.NewRequestWithContext(ctx, "POST", selectedEndpoint.EndpointURL, bytes.NewReader(requestBody))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := d.httpClient.Do(req)
	if err != nil {
		// Update endpoint health status
		d.endpointManager.UpdateEndpointHealth(selectedEndpoint.ID, "error")
		return nil, err
	}
	defer resp.Body.Close()

	// Read the response
	responseBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	// Update endpoint health status to healthy
	d.endpointManager.UpdateEndpointHealth(selectedEndpoint.ID, "healthy")

	return responseBody, nil
}

// ForwardToViperNetwork handles forwarding requests to the Viper Network,
// translating between JSON-RPC and Viper Network formats
func (d *Dispatcher) ForwardToViperNetwork(ctx context.Context, requestBody []byte) ([]byte, error) {
	// Convert from JSON-RPC format to Viper Network format
	requestType, viperRequest, err := ConvertJSONRPCToViperFormat(requestBody)
	if err != nil {
		return nil, err
	}

	// Send the request to Viper Network
	viperResponse, err := d.viperNetworkHandler.HandleViperRequest(ctx, requestType, viperRequest)
	if err != nil {
		return nil, err
	}

	// Convert the response back to JSON-RPC format
	jsonRPCResponse, err := ConvertViperResponseToJSONRPC(viperResponse, requestBody)
	if err != nil {
		return nil, err
	}

	return jsonRPCResponse, nil
}
