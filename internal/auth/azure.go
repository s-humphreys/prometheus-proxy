package auth

import (
	"context"
	"errors"
	"fmt"

	"github.com/s-humphreys/prometheus-proxy/internal/logger"

	"github.com/AzureAD/microsoft-authentication-library-for-go/apps/confidential"
)

var (
	errUnimplementedAzAuthMethod        = errors.New("unimplemented azure authentication method, please provide a client secret")
	errConfidentialClientNotInitialized = errors.New("confidential client not initialized")

	azureTenantPrefix = "https://login.microsoftonline.com/"
)

type AzureClient struct {
	TenantId     string
	ClientId     string
	ClientSecret string
	Logger       *logger.Logger
	confClient   *confidential.Client
}

// Initiliases the Azure client using the provided credentials
func (ac *AzureClient) InitClient(logger *logger.Logger) error {
	logger.Info("using azure client for authentication", "client_id", ac.ClientId, "tenant_id", ac.TenantId)
	ac.Logger = logger

	if ac.ClientSecret == "" {
		return errUnimplementedAzAuthMethod
	}

	confClient, err := newConfidentialClient(ac)
	if err != nil {
		return err
	}
	ac.confClient = confClient

	return nil
}

// Authenticates with Azure using App Registration credentials and returns an access token
func (ac *AzureClient) AcquireToken(ctx context.Context) (string, error) {
	if ac.ClientSecret != "" {
		return getConfidentialClientToken(ac, ctx)
	}

	// TODO - Implement Managed Identity authentication
	return "", errEmptyToken
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

// Uses an App Registration to source a token from Azure AD
func getConfidentialClientToken(client *AzureClient, ctx context.Context) (string, error) {
	cLog := client.Logger.With(
		"client_id", client.ClientId,
		"tenant_id", client.TenantId,
	)

	cLog.Debug("acquiring azure token using app registration credentials")
	if client.confClient == nil {
		return "", errConfidentialClientNotInitialized
	}

	scopes := []string{"https://prometheus.monitor.azure.com/.default"}
	result, err := client.confClient.AcquireTokenSilent(ctx, scopes)

	if result.AccessToken != "" {
		cLog.Debug("acquired azure token using cache/refresh")
	}

	if err != nil {
		cLog.Warn("failed acquire azure cache/refresh token, proceeding to acquire a new token", "error", err)
		result, err = client.confClient.AcquireTokenByCredential(ctx, scopes)
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
