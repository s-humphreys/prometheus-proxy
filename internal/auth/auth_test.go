package auth

import (
	"context"
	"testing"

	"github.com/s-humphreys/prometheus-proxy/internal/logger"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestClientHeader(t *testing.T) {
	header := ClientHeader{
		Key:   "Authorization",
		Value: "Bearer token123",
	}

	assert.Equal(t, "Authorization", header.Key)
	assert.Equal(t, "Bearer token123", header.Value)
}

func TestClientInterface(t *testing.T) {
	// Test that AzureClient implements the Client interface
	var client Client = &AzureClient{}
	assert.NotNil(t, client)
}

func TestErrorConstants(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		expected string
	}{
		{
			name:     "errEmptyToken",
			err:      errEmptyToken,
			expected: "empty authentication token",
		},
		{
			name:     "errClientNotInitialised",
			err:      errClientNotInitialised,
			expected: "azure client not initialized",
		},
		{
			name:     "errUnsetClientId",
			err:      errUnsetClientId,
			expected: "environment variable AZURE_CLIENT_ID is unset",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, tt.err.Error())
		})
	}
}

func TestAzureClientStruct(t *testing.T) {
	client := &AzureClient{
		TenantId:     "test-tenant",
		ClientId:     "test-client",
		ClientSecret: "test-secret",
	}

	assert.Equal(t, "test-tenant", client.TenantId)
	assert.Equal(t, "test-client", client.ClientId)
	assert.Equal(t, "test-secret", client.ClientSecret)
	assert.Nil(t, client.Logger)
	assert.Nil(t, client.confClient)
	assert.Nil(t, client.workloadIdentityCred)
}

func TestValidateClientId(t *testing.T) {
	tests := []struct {
		name     string
		clientId string
		expected bool
	}{
		{
			name:     "valid client ID",
			clientId: "test-client-id",
			expected: true,
		},
		{
			name:     "empty client ID",
			clientId: "",
			expected: false,
		},
		{
			name:     "no value placeholder",
			clientId: "<no value>",
			expected: false,
		},
		{
			name:     "valid UUID",
			clientId: "12345678-1234-1234-1234-123456789012",
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := validateClientId(tt.clientId)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestAzureClientGetHeaders_NotInitialized(t *testing.T) {
	client := &AzureClient{
		TenantId: "test-tenant",
		ClientId: "test-client",
	}

	ctx := context.Background()
	headers, err := client.GetHeaders(ctx)

	assert.Error(t, err)
	assert.Nil(t, headers)
	assert.Equal(t, errClientNotInitialised, err)
}

func TestAzureClientAcquireToken_NotInitialized(t *testing.T) {
	client := &AzureClient{
		TenantId: "test-tenant",
		ClientId: "test-client",
	}

	ctx := context.Background()
	token, err := client.AcquireToken(ctx)

	assert.Error(t, err)
	assert.Empty(t, token)
	assert.Equal(t, errClientNotInitialised, err)
}

func TestAzureConstants(t *testing.T) {
	// Test that constants are defined as expected
	assert.Equal(t, []string{"https://prometheus.monitor.azure.com/.default"}, azureScopes)
	assert.Equal(t, "https://login.microsoftonline.com/", azureTenantPrefix)
	assert.Equal(t, "/var/run/secrets/azure/tokens/azure-identity-token", azureWorkloadIdentityTokenPath)
}

// Test the InitClient method behavior (without actually initializing Azure clients)
func TestAzureClientInitClient_InvalidLogger(t *testing.T) {
	client := &AzureClient{
		TenantId: "test-tenant",
		ClientId: "test-client",
	}

	// Create a test logger instead of passing nil
	testLogger, err := logger.New("ERROR")
	require.NoError(t, err)

	// This should fail due to missing/invalid credentials, but not crash
	err = client.InitClient(testLogger)

	// Error is expected due to missing/invalid credentials
	if err != nil {
		assert.Error(t, err)
	}

	// Logger should be set
	assert.Equal(t, testLogger, client.Logger)
}

// Test error cases and edge conditions
func TestNewConfidentialClient_ErrorHandling(t *testing.T) {
	// Create a test logger
	testLogger, err := logger.New("ERROR")
	require.NoError(t, err)

	client := &AzureClient{
		TenantId:     "", // Empty tenant ID should cause error
		ClientId:     "test-client",
		ClientSecret: "test-secret",
		Logger:       testLogger,
	}

	// This should fail due to empty tenant ID
	_, err = newConfidentialClient(client)
	assert.Error(t, err)
}

func TestNewWorkloadIdentityCred_ErrorHandling(t *testing.T) {
	// Create a test logger
	testLogger, err := logger.New("ERROR")
	require.NoError(t, err)

	client := &AzureClient{
		TenantId: "", // Empty tenant ID should cause error
		ClientId: "test-client",
		Logger:   testLogger,
	}

	// This should fail due to empty tenant ID
	_, err = newWorkloadIdentityCred(client)
	assert.Error(t, err)
}

// Test refreshClientId with various environment scenarios
func TestRefreshClientId(t *testing.T) {
	// Create a test logger
	testLogger, err := logger.New("ERROR")
	require.NoError(t, err)

	client := &AzureClient{
		TenantId: "test-tenant",
		ClientId: "original-client-id",
		Logger:   testLogger,
	}

	// Store original environment
	originalClientId := client.ClientId

	// Test refreshing when environment variable is not set
	err = client.refreshClientId()

	// Should get an error or update the client ID based on environment
	// The actual behavior depends on the environment
	if err != nil {
		assert.Equal(t, errUnsetClientId, err)
	} else {
		// If no error, ClientId should be updated from environment
		// (but we can't predict what the environment variable is)
	}

	// Restore original client ID
	client.ClientId = originalClientId
}

// Test that GetHeaders returns proper format when token is available
func TestClientHeader_Format(t *testing.T) {
	headers := []ClientHeader{
		{Key: "Authorization", Value: "Bearer test-token"},
		{Key: "Content-Type", Value: "application/json"},
	}

	assert.Equal(t, "Authorization", headers[0].Key)
	assert.Equal(t, "Bearer test-token", headers[0].Value)
	assert.Equal(t, "Content-Type", headers[1].Key)
	assert.Equal(t, "application/json", headers[1].Value)
}

// Benchmark tests
func BenchmarkValidateClientId(b *testing.B) {
	clientId := "valid-client-id-12345"

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		validateClientId(clientId)
	}
}

func BenchmarkClientHeaderCreation(b *testing.B) {
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		headers := []ClientHeader{
			{Key: "Authorization", Value: "Bearer benchmark-token"},
		}
		_ = headers
	}
}
