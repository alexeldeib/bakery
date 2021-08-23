package main

import (
	"fmt"

	"github.com/Azure/go-autorest/autorest"
	"github.com/Azure/go-autorest/autorest/adal"
	"github.com/Azure/go-autorest/autorest/azure"
)

// MSIConfig provides the options to get a bearer authorizer through MSI.
type MSIConfig struct {
	Resource           string
	IdentityResourceID string
}

// NewMSIConfig creates an MSIConfig object configured to obtain an Authorizer through MSI.
func NewMSIConfig(identity string) MSIConfig {
	return MSIConfig{
		IdentityResourceID: identity,
		Resource:           azure.PublicCloud.ResourceManagerEndpoint,
	}
}

// ServicePrincipalToken creates a ServicePrincipalToken from MSI.
func (mc MSIConfig) ServicePrincipalToken() (*adal.ServicePrincipalToken, error) {
	spToken, err := adal.NewServicePrincipalTokenFromManagedIdentity(mc.Resource, &adal.ManagedIdentityOptions{
		ClientID: mc.IdentityResourceID,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to get oauth token from MSI: %v", err)
	}
	return spToken, nil
}

// Authorizer gets the authorizer from MSI.
func (mc MSIConfig) Authorizer() (autorest.Authorizer, error) {
	spToken, err := mc.ServicePrincipalToken()
	if err != nil {
		return nil, err
	}

	return autorest.NewBearerAuthorizer(spToken), nil
}
