package auth

import (
	"context"
	"errors"
	"fmt"
	"os"

	"github.com/s-humphreys/prometheus-proxy/internal/logger"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore/policy"
	"github.com/Azure/azure-sdk-for-go/sdk/azidentity"
	"github.com/AzureAD/microsoft-authentication-library-for-go/apps/confidential"
)

var (
	errClientNotInitialised = errors.New("azure client not initialized")
	errUnsetClientId        = errors.New("environment variable AZURE_CLIENT_ID is unset")

	azureScopes                    = []string{"https://prometheus.monitor.azure.com/.default"}
	azureTenantPrefix              = "https://login.microsoftonline.com/"
	azureWorkloadIdentityTokenPath = "/var/run/secrets/azure/tokens/azure-identity-token"
)

type AzureClient struct {
	TenantId             string
	ClientId             string
	ClientSecret         string
	Logger               *logger.Logger
	confClient           *confidential.Client
	workloadIdentityCred *azidentity.WorkloadIdentityCredential
}

// Initiliases the Azure client using the provided credentials
func (ac *AzureClient) InitClient(logger *logger.Logger) error {
	logger.Info("using azure client for authentication", "client_id", ac.ClientId, "tenant_id", ac.TenantId)
	ac.Logger = logger

	// Use App Registration auth if client secret is provided
	if ac.ClientSecret != "" {
		confClient, err := newConfidentialClient(ac)
		if err != nil {
			return err
		}
		ac.confClient = confClient
		return nil
	}

	// Use Managed (workload) Identity auth
	workloadIdentityCred, err := newWorkloadIdentityCred(ac)
	if err != nil {
		return err
	}
	ac.workloadIdentityCred = workloadIdentityCred

	return nil
}

// Authenticates with Azure and returns an access token
func (ac *AzureClient) AcquireToken(ctx context.Context) (string, error) {
	if ac.confClient != nil {
		return getConfidentialClientToken(ac, ctx)
	}

	if ac.workloadIdentityCred != nil {
		return getWorkloadIdentityToken(ac, ctx)
	}

	return "", errClientNotInitialised
}

// Returns the headers required for authenticating requests to Azure Managed Prometheus
func (ac *AzureClient) GetHeaders(ctx context.Context) ([]ClientHeader, error) {
	token, err := ac.AcquireToken(ctx)
	if err != nil {
		return nil, err
	}

	return []ClientHeader{
		{Key: "Authorization", Value: fmt.Sprintf("Bearer %s", token)},
	}, nil
}

// Updates the Azure Client ID in case Azure does not set the value automatically before first use
func (ac *AzureClient) refreshClientId() error {
	clientId := os.Getenv("AZURE_CLIENT_ID")
	if ok := validateClientId(clientId); !ok {
		ac.Logger.Error("AZURE_CLIENT_ID environment variable is unset", "client_id", clientId)
		return errUnsetClientId
	}

	ac.ClientId = clientId
	return nil
}

// Validates the Azure Client ID is set and not empty
func validateClientId(clientId string) bool {
	if clientId == "" || clientId == "<no value>" {
		return false
	}
	return true
}

// Creates a new confidential client for Azure authentication
// this ensures token acquisition uses cache and refresh tokens
func newConfidentialClient(client *AzureClient) (*confidential.Client, error) {
	client.Logger.Debug("creating new confidential client", "client_id", client.ClientId, "tenant_id", client.TenantId)
	cred, err := confidential.NewCredFromSecret(client.ClientSecret)
	if err != nil {
		return nil, err
	}

	tenant := azureTenantPrefix + client.TenantId
	confClient, err := confidential.New(tenant, client.ClientId, cred)
	if err != nil {
		return nil, err
	}

	return &confClient, nil
}

// Creates a new confidential client for Azure authentication
// this ensures token acquisition uses cache and refresh tokens
func newWorkloadIdentityCred(client *AzureClient) (*azidentity.WorkloadIdentityCredential, error) {
	client.Logger.Debug("creating new workload identity credential", "client_id", client.ClientId, "tenant_id", client.TenantId)

	if ok := validateClientId(client.ClientId); !ok {
		err := client.refreshClientId()
		if err != nil {
			return nil, err
		}
	}

	cred, err := azidentity.NewWorkloadIdentityCredential(&azidentity.WorkloadIdentityCredentialOptions{
		ClientID:      client.ClientId,
		TenantID:      client.TenantId,
		TokenFilePath: azureWorkloadIdentityTokenPath,
	})
	if err != nil {
		return nil, err
	}

	return cred, nil
}

// Uses an App Registration to source a token from Azure AD
func getConfidentialClientToken(client *AzureClient, ctx context.Context) (string, error) {
	l := client.Logger.With(
		"client_id", client.ClientId,
		"tenant_id", client.TenantId,
	)

	l.Debug("acquiring azure token using app registration credentials")
	result, err := client.confClient.AcquireTokenSilent(ctx, azureScopes)
	if result.AccessToken != "" {
		l.Debug("acquired azure token using cache/refresh")
	}

	if err != nil {
		l.Warn("failed to acquire azure cache/refresh token, proceeding to acquire a new token", "error", err)
		result, err = client.confClient.AcquireTokenByCredential(ctx, azureScopes)
		if err != nil {
			l.Error("failed to acquire azure token", "error", err)
			return "", err
		}
	}

	if result.AccessToken == "" {
		l.Error("acquired empty azure token")
		return "", errEmptyToken
	}

	l.Debug("acquired azure token successfully")
	return result.AccessToken, nil
}

// Uses a Managed Identity to source a token from Azure AD
func getWorkloadIdentityToken(client *AzureClient, ctx context.Context) (string, error) {
	l := client.Logger.With(
		"client_id", client.ClientId,
		"tenant_id", client.TenantId,
	)

	l.Debug("acquiring azure token using workload identity credentials")
	token, err := client.workloadIdentityCred.GetToken(ctx, policy.TokenRequestOptions{Scopes: azureScopes})
	if err != nil {
		l.Error("failed to acquire azure token", "error", err)
		return "", err
	}

	if token.Token == "" {
		l.Error("acquired empty azure token")
		return "", errEmptyToken
	}

	l.Debug("acquired azure token successfully")
	return token.Token, nil
}
