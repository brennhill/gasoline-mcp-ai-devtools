package transport

// ExtensionTransportProvider defines the shared endpoint identity/mutator contract
// used by extension and server transport implementations.
type ExtensionTransportProvider interface {
	ID() string
	SetEndpoint(endpoint string)
	Endpoint() string
}

// HTTPExtensionTransportProvider is the HTTP transport implementation.
type HTTPExtensionTransportProvider struct {
	endpoint string
}

// NewHTTPExtensionTransportProvider creates a HTTP extension transport provider.
func NewHTTPExtensionTransportProvider(endpoint string) *HTTPExtensionTransportProvider {
	return &HTTPExtensionTransportProvider{endpoint: endpoint}
}

// ID returns the provider identifier.
func (p *HTTPExtensionTransportProvider) ID() string {
	return "http"
}

// SetEndpoint updates the current endpoint.
func (p *HTTPExtensionTransportProvider) SetEndpoint(endpoint string) {
	p.endpoint = endpoint
}

// Endpoint returns the current endpoint.
func (p *HTTPExtensionTransportProvider) Endpoint() string {
	return p.endpoint
}
