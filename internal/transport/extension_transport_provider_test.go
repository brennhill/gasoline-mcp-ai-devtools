package transport

import "testing"

// extensionTransportProviderContract locks the minimal cross-language provider contract.
type extensionTransportProviderContract interface {
	ID() string
	SetEndpoint(endpoint string)
	Endpoint() string
}

func TestHTTPExtensionTransportProvider_ImplementsContract(t *testing.T) {
	var _ extensionTransportProviderContract = NewHTTPExtensionTransportProvider("http://127.0.0.1:7777")
}

func TestHTTPExtensionTransportProvider_ID(t *testing.T) {
	provider := NewHTTPExtensionTransportProvider("http://127.0.0.1:7777")
	if got := provider.ID(); got != "http" {
		t.Fatalf("provider.ID() = %q, want %q", got, "http")
	}
}

func TestHTTPExtensionTransportProvider_EndpointMutators(t *testing.T) {
	provider := NewHTTPExtensionTransportProvider("http://127.0.0.1:7777")
	provider.SetEndpoint("http://127.0.0.1:8888")
	if got := provider.Endpoint(); got != "http://127.0.0.1:8888" {
		t.Fatalf("provider.Endpoint() = %q, want %q", got, "http://127.0.0.1:8888")
	}
}

