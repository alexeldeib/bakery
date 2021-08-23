package main

import (
	"context"
	"errors"

	"github.com/Azure/azure-sdk-for-go/services/containerservice/mgmt/2021-03-01/containerservice"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

type KubeconfigGetter func(ctx context.Context, subscriptionID, resourceGroup, clusterName string) (*rest.Config, error)
type KubeconfigGetterFactory func(client containerservice.ManagedClustersClient) KubeconfigGetter

func NewKubeconfigGetter(client containerservice.ManagedClustersClient) KubeconfigGetter {
	return func(ctx context.Context, _, resourceGroup, clusterName string) (*rest.Config, error) {
		credentialList, err := client.ListClusterAdminCredentials(ctx, resourceGroup, clusterName)
		if err != nil {
			return nil, err
		}

		if credentialList.Kubeconfigs == nil || len(*credentialList.Kubeconfigs) < 1 {
			return nil, errors.New("no kubeconfigs available for the managed cluster cluster")
		}

		return clientcmd.RESTConfigFromKubeConfig(*(*credentialList.Kubeconfigs)[0].Value)
	}
}
