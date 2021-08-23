package main

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/Azure/azure-sdk-for-go/services/compute/mgmt/2021-04-01/compute"
	"github.com/Azure/go-autorest/autorest"
	"github.com/Azure/go-autorest/autorest/to"
	"github.com/hashicorp/go-multierror"
	"github.com/sanity-io/litter"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

//go:generate gomodifytags -file $GOFILE -struct snapshot -add-tags json -transform camelcase -w
//go:generate gomodifytags -file $GOFILE -struct managedCluster -add-tags json -transform camelcase -w
//go:generate gomodifytags -file $GOFILE -struct sharedImageGallery -add-tags json -transform camelcase -w

type snapshot struct {
	NodeName           string             `json:"nodeName"`
	ManagedCluster     managedCluster     `json:"managedCluster"`
	SharedImageGallery sharedImageGallery `json:"sharedImageGallery"`
}

type managedCluster struct {
	SubscriptionID    string `json:"subscriptionID"`
	ResourceGroupName string `json:"resourceGroupName"`
	ClusterName       string `json:"clusterName"`
}

type sharedImageGallery struct {
	SubscriptionID    string `json:"subscriptionID"`
	ResourceGroupName string `json:"resourceGroupName"`
	GalleryName       string `json:"galleryName"`
	ImageDefinition   string `json:"imageDefinition"`
	ImageVersion      string `json:"imageVersion"`
}

// type apiError struct {
// 	Err  error `json:"err"`
// 	Code int32 `json:"code"`
// }

// func (a *apiError) Error() string {
// 	return a.Err.Error()
// }

func (c *snapshot) isValid() error {
	var error *multierror.Error
	if c.NodeName == "" {
		error = multierror.Append(error, fmt.Errorf("nodeName cannot be empty"))
	}
	if c.ManagedCluster.SubscriptionID == "" {
		error = multierror.Append(error, fmt.Errorf("managedCluster.subscriptionID cannot be empty"))
	}
	if c.ManagedCluster.ResourceGroupName == "" {
		error = multierror.Append(error, fmt.Errorf("managedCluster.resourceGroupName cannot be empty"))
	}
	if c.ManagedCluster.ClusterName == "" {
		error = multierror.Append(error, fmt.Errorf("managedCluster.clusterName cannot be empty"))
	}
	if c.SharedImageGallery.SubscriptionID == "" {
		error = multierror.Append(error, fmt.Errorf("sharedImageGallery.subscriptionID cannot be empty"))
	}
	if c.SharedImageGallery.ResourceGroupName == "" {
		error = multierror.Append(error, fmt.Errorf("sharedImageGallery.resourceGroupName cannot be empty"))
	}
	if c.SharedImageGallery.GalleryName == "" {
		error = multierror.Append(error, fmt.Errorf("sharedImageGallery.galleryName cannot be empty"))
	}
	if c.SharedImageGallery.ImageDefinition == "" {
		error = multierror.Append(error, fmt.Errorf("sharedImageGallery.imageDefinition cannot be empty"))
	}
	if c.SharedImageGallery.ImageVersion == "" {
		error = multierror.Append(error, fmt.Errorf("sharedImageGallery.imageVersion cannot be empty"))
	}
	return error.ErrorOrNil()
}

type Bakery struct {
	Cloud *AzureClients
	Auth  autorest.Authorizer
	Kube  client.Client
}

func (b *Bakery) createSnapshot(args *snapshot) error {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*600)
	defer cancel()
	var node corev1.Node
	var key = types.NamespacedName{
		Name: args.NodeName,
	}
	if err := b.Kube.Get(ctx, key, &node); err != nil {
		// if status := apierror.APIStatus(nil); errors.As(err, &status) {
		// 	return &apiError{
		// 		Err:  err,
		// 		Code: status.Status().Code,
		// 	}
		// }
		return err
	}

	// Untrimmed ID: azure:///subscriptions/<subscription id>/resourceGroups/k8sblogkk1/providers/Microsoft.Compute/virtualMachineScaleSets/k8s-agentpool1-92998111-vmss/virtualMachines/0
	// Trimmed ID: /subscriptions/<subscription id>/resourceGroups/k8sblogkk1/providers/Microsoft.Compute/virtualMachineScaleSets/k8s-agentpool1-92998111-vmss/virtualMachines/0
	// Tokenized ID: [subscriptions, <ID>, resourceGroups, <RG>, providers, Microsoft.Compute, virtualMachineScaleSets, <VMSS>, virtualMachines, 0]
	// sub: tokens[1], rg: tokens[3], vmss: tokens[7], instance: tokens[9]
	subscriptionID, resourceGroup, scaleSet, instance, err := tokenizeProviderID(node.Spec.ProviderID)
	if err != nil {
		return fmt.Errorf("failed to tokenize spec.providerID. %w", err)
	}
	gallerySub := args.SharedImageGallery.SubscriptionID
	galleryName := args.SharedImageGallery.GalleryName
	galleryResourceGroup := args.SharedImageGallery.ResourceGroupName
	imageDefinitionName := args.SharedImageGallery.ImageDefinition
	imageVersionName := args.SharedImageGallery.ImageVersion

	srcclient := NewClients(subscriptionID, b.Auth)
	dstclient := NewClients(gallerySub, b.Auth)

	vm, err := srcclient.VirtualMachineScaleSetVMs.Get(ctx, resourceGroup, scaleSet, instance, compute.InstanceViewTypesInstanceView)
	if err != nil {
		return fmt.Errorf("failed to start vm deallocation. %w", err)
	}

	if vm.InstanceView == nil || vm.InstanceView.Statuses == nil {
		return fmt.Errorf("expected to find powerState and provisioninState statuses")
	}

	var isDeallocated bool
	for _, status := range *vm.InstanceView.Statuses {
		litter.Dump(status)
		if status.Code != nil && *status.Code == "PowerState/deallocated" {
			isDeallocated = true
		}
	}

	if !isDeallocated {
		vmFuture, err := srcclient.VirtualMachineScaleSetVMs.Deallocate(ctx, resourceGroup, scaleSet, instance)
		if err != nil {
			return fmt.Errorf("failed to start vm deallocation. %w", err)
		}

		if err := vmFuture.WaitForCompletionRef(ctx, srcclient.VirtualMachineScaleSetVMs.Client); err != nil {
			return fmt.Errorf("failed to wait for vm deallocation future. %w", err)
		}

		result, err := vmFuture.Result(srcclient.VirtualMachineScaleSetVMs)
		if err != nil {
			return fmt.Errorf("failed to get result of deallocation. %w", err)
		}

		if result.StatusCode != http.StatusOK {
			return fmt.Errorf("expected http 200 deallocating vm, got status code %d", result.StatusCode)
		}
	}

	vm, err = srcclient.VirtualMachineScaleSetVMs.Get(ctx, resourceGroup, scaleSet, instance, compute.InstanceViewTypesInstanceView)
	if err != nil {
		return fmt.Errorf("failed to start vm deallocation. %w", err)
	}

	if vm.StorageProfile == nil || vm.StorageProfile.OsDisk == nil || vm.StorageProfile.OsDisk.ManagedDisk == nil || vm.StorageProfile.OsDisk.ManagedDisk.ID == nil {
		return fmt.Errorf("vm os disk managed disk id is nil, cannot snapshot")
	}
	osDiskID := *vm.StorageProfile.OsDisk.ManagedDisk.ID

	_, err = dstclient.Galleries.Get(ctx, galleryResourceGroup, galleryName, compute.SelectPermissionsPermissions)
	if err != nil && !NotFound(err) {
		return fmt.Errorf("failed to get existing gallery. %w", err)
	}

	if NotFound(err) {
		galleryFuture, err := dstclient.Galleries.CreateOrUpdate(ctx, resourceGroup, galleryName, compute.Gallery{Location: vm.Location})
		if err != nil {
			return fmt.Errorf("failed to wait for gallery creation")
		}
		if err := galleryFuture.WaitForCompletionRef(ctx, dstclient.VirtualMachineScaleSetVMs.Client); err != nil {
			return fmt.Errorf("failed to wait for vm deallocation future. %w", err)
		}

		result, err := galleryFuture.Result(dstclient.Galleries)
		if err != nil {
			return fmt.Errorf("expected http 200 creating gallery, got status code %d", result.StatusCode)
		}
	}

	_, err = dstclient.GalleryImages.Get(ctx, galleryResourceGroup, galleryName, imageDefinitionName)
	if err != nil && !NotFound(err) {
		return fmt.Errorf("failed to get existing image definition. %w", err)
	}

	if NotFound(err) {
		imageDefinition := compute.GalleryImage{
			Location: vm.Location,
			GalleryImageProperties: &compute.GalleryImageProperties{
				OsType: compute.OperatingSystemTypesLinux,
				Identifier: &compute.GalleryImageIdentifier{
					Publisher: to.StringPtr("aks"),
					Offer:     to.StringPtr("node"),
					Sku:       to.StringPtr("ubuntu"),
				},
			},
		}

		imageDefinitionFuture, err := dstclient.GalleryImages.CreateOrUpdate(ctx, galleryResourceGroup, galleryName, imageDefinitionName, imageDefinition)
		if err != nil {
			return fmt.Errorf("failed to wait for gallery creation. %w", err)
		}

		if err := imageDefinitionFuture.WaitForCompletionRef(ctx, dstclient.VirtualMachineScaleSetVMs.Client); err != nil {
			return fmt.Errorf("failed to wait for vm deallocation future. %w", err)
		}

		result, err := imageDefinitionFuture.Result(dstclient.GalleryImages)
		if err != nil {
			return fmt.Errorf("expected http 200 creating gallery, got status code %d", result.StatusCode)
		}
	}

	_, err = dstclient.GalleryImageVersions.Get(ctx, galleryResourceGroup, galleryName, imageDefinitionName, imageVersionName, "")
	if err != nil && !NotFound(err) {
		return fmt.Errorf("failed to get existing image version. %w", err)
	}

	// attempting to overwrite existing image version. reject.
	if err == nil {
		return fmt.Errorf("image version already exists. refusing to overwrite")
	}

	imageVersionInput := compute.GalleryImageVersion{
		Location: vm.Location,
		GalleryImageVersionProperties: &compute.GalleryImageVersionProperties{
			StorageProfile: &compute.GalleryImageVersionStorageProfile{
				OsDiskImage: &compute.GalleryOSDiskImage{
					Source: &compute.GalleryArtifactVersionSource{
						ID: &osDiskID,
					},
				},
			},
			PublishingProfile: &compute.GalleryImageVersionPublishingProfile{
				TargetRegions: &[]compute.TargetRegion{
					{
						Name:               vm.Location,
						StorageAccountType: compute.StorageAccountTypeStandardLRS,
					},
				},
			},
		},
	}

	imageVersionFuture, err := dstclient.GalleryImageVersions.CreateOrUpdate(ctx, galleryResourceGroup, galleryName, imageDefinitionName, imageVersionName, imageVersionInput)
	if err != nil {
		return fmt.Errorf("failed to wait for gallery creation")
	}

	if err := imageVersionFuture.WaitForCompletionRef(ctx, dstclient.VirtualMachineScaleSetVMs.Client); err != nil {
		return fmt.Errorf("failed to wait for vm deallocation future. %w", err)
	}

	output, err := imageVersionFuture.Result(dstclient.GalleryImageVersions)
	if err != nil {
		return fmt.Errorf("expected http 200 creating gallery, got status code %d", output.StatusCode)
	}

	return nil
}

// func (b *bakery) getSnapshot()                                 {}
// func (b *bakery) deleteSnapshot() apiError { return nil }

func tokenizeProviderID(providerID string) (subscriptionID, resourceGroup, scaleSet, instance string, err error) {
	providerID = strings.TrimPrefix(providerID, "azure://")
	providerIDTokens := strings.Split(providerID, "/")[1:]
	providerIDTokenMap := make(map[int]string)
	expectedTokenMap := map[int]string{
		0: "subscriptions",
		2: "resourceGroups",
		4: "providers",
		5: "Microsoft.Compute",
		6: "virtualMachineScaleSets",
		8: "virtualMachines",
	}

	if l := len(providerIDTokens); l != 10 {
		return "", "", "", "", fmt.Errorf("invalid spec.providerID %s. expected 10 token segments, found %d", providerID, l)

	}

	for i, token := range providerIDTokens {
		providerIDTokenMap[i] = token
	}

	for k, v := range expectedTokenMap {
		if !strings.EqualFold(v, providerIDTokenMap[k]) {
			return "", "", "", "", fmt.Errorf("invalid spec.providerID %s. expected token %d to be %s", providerID, k, v)
		}
	}

	subscriptionID = providerIDTokenMap[1]
	resourceGroup = providerIDTokenMap[3]
	scaleSet = providerIDTokenMap[7]
	instance = providerIDTokenMap[9]
	return subscriptionID, resourceGroup, scaleSet, instance, nil
}
