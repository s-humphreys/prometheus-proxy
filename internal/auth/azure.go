package auth

import (
	"context"
	"errors"
	"fmt"

	"github.com/s-humphreys/prometheus-proxy/internal/logger"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore/policy"
	"github.com/Azure/azure-sdk-for-go/sdk/azidentity"
	"github.com/AzureAD/microsoft-authentication-library-for-go/apps/confidential"
)

var (
	errClientNotInitialised = errors.New("azure client not initialized")

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
	cLog := client.Logger.With(
		"client_id", client.ClientId,
		"tenant_id", client.TenantId,
	)

	cLog.Debug("acquiring azure token using app registration credentials")

	result, err := client.confClient.AcquireTokenSilent(ctx, azureScopes)

	if result.AccessToken != "" {
		cLog.Debug("acquired azure token using cache/refresh")
	}

	if err != nil {
		cLog.Warn("failed acquire azure cache/refresh token, proceeding to acquire a new token", "error", err)
		result, err = client.confClient.AcquireTokenByCredential(ctx, azureScopes)
		if err != nil {
			cLog.Error("failed to acquire azure token", "error", err)
			return "", err
		}
	}

	if result.AccessToken == "" {
		cLog.Error("acquired empty azure token")
		return "", errEmptyToken
	}

	cLog.Debug("acquired azure token successfully")
	return result.AccessToken, nil
}

// Uses a Managed Identity to source a token from Azure AD
func getWorkloadIdentityToken(client *AzureClient, ctx context.Context) (string, error) {
	cLog := client.Logger.With(
		"client_id", client.ClientId,
		"tenant_id", client.TenantId,
	)

	cLog.Debug("acquiring azure token using workload identity credentials")

	token, err := client.workloadIdentityCred.GetToken(ctx, policy.TokenRequestOptions{Scopes: azureScopes})
	if err != nil {
		cLog.Error("failed to acquire azure token", "error", err)
		return "", err
	}

	if token.Token == "" {
		cLog.Error("acquired empty azure token")
		return "", errEmptyToken
	}

	cLog.Debug("acquired azure token successfully")
	return token.Token, nil
}
