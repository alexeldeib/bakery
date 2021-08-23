package main

import (
	"fmt"
	"net/http"
	"net/http/httputil"

	"github.com/Azure/azure-sdk-for-go/services/compute/mgmt/2021-04-01/compute"
	"github.com/Azure/go-autorest/autorest"
)

type AzureClients struct {
	Disks                     compute.DisksClient
	Snapshots                 compute.SnapshotsClient
	Galleries                 compute.GalleriesClient
	GalleryImages             compute.GalleryImagesClient
	GalleryImageVersions      compute.GalleryImageVersionsClient
	VirtualMachines           compute.VirtualMachinesClient
	VirtualMachineScaleSets   compute.VirtualMachineScaleSetsClient
	VirtualMachineScaleSetVMs compute.VirtualMachineScaleSetVMsClient
}

func NewClients(sub string, auth autorest.Authorizer) *AzureClients {
	return &AzureClients{
		Disks:                     NewDisksClient(sub, auth),
		Galleries:                 NewGalleriesClient(sub, auth),
		GalleryImages:             NewGalleryImagesClient(sub, auth),
		GalleryImageVersions:      NewGalleryImageVersionsClient(sub, auth),
		Snapshots:                 NewSnapshotsClient(sub, auth),
		VirtualMachineScaleSetVMs: NewVirtualMachineScaleSetVMsClient(sub, auth),
	}
}

// NewDisksClient returns an authenticated client using the provided authorizer factory.
func NewDisksClient(sub string, authorizer autorest.Authorizer) compute.DisksClient {
	client := compute.NewDisksClient(sub)
	client.Authorizer = authorizer
	client.RequestInspector = LogRequest()
	client.ResponseInspector = LogResponse()
	return client
}

// NewGalleriesClient returns an authenticated client using the provided authorizer factory.
func NewGalleriesClient(sub string, authorizer autorest.Authorizer) compute.GalleriesClient {
	client := compute.NewGalleriesClient(sub)
	client.Authorizer = authorizer
	client.RequestInspector = LogRequest()
	client.ResponseInspector = LogResponse()
	return client
}

// NewGalleryImagesClient returns an authenticated client using the provided authorizer factory.
func NewGalleryImagesClient(sub string, authorizer autorest.Authorizer) compute.GalleryImagesClient {
	client := compute.NewGalleryImagesClient(sub)
	client.Authorizer = authorizer
	return client
}

// NewGalleryImageVersionsClient returns an authenticated client using the provided authorizer factory.
func NewGalleryImageVersionsClient(sub string, authorizer autorest.Authorizer) compute.GalleryImageVersionsClient {
	client := compute.NewGalleryImageVersionsClient(sub)
	client.Authorizer = authorizer
	client.RequestInspector = LogRequest()
	client.ResponseInspector = LogResponse()
	return client
}

// NewSnapshotsClient returns an authenticated client using the provided authorizer factory.
func NewSnapshotsClient(sub string, authorizer autorest.Authorizer) compute.SnapshotsClient {
	client := compute.NewSnapshotsClient(sub)
	client.Authorizer = authorizer
	return client
}

// NewVirtualMachinesClient returns an authenticated client using the provided authorizer factory.
func NewVirtualMachinesClient(sub string, authorizer autorest.Authorizer) compute.VirtualMachinesClient {
	client := compute.NewVirtualMachinesClient(sub)
	client.Authorizer = authorizer
	return client
}

// NewVirtualMachineScaleSetsClient returns an authenticated client using the provided authorizer factory.
func NewVirtualMachineScaleSetsClient(sub string, authorizer autorest.Authorizer) compute.VirtualMachineScaleSetsClient {
	client := compute.NewVirtualMachineScaleSetsClient(sub)
	client.Authorizer = authorizer
	client.RequestInspector = LogRequest()
	client.ResponseInspector = LogResponse()
	return client
}

// NewVirtualMachineScaleSetVMsClient returns an authenticated client using the provided authorizer factory.
func NewVirtualMachineScaleSetVMsClient(sub string, authorizer autorest.Authorizer) compute.VirtualMachineScaleSetVMsClient {
	client := compute.NewVirtualMachineScaleSetVMsClient(sub)
	client.Authorizer = authorizer
	client.RequestInspector = LogRequest()
	client.ResponseInspector = LogResponse()
	return client
}

// LogRequest logs full autorest requests for any Azure client.
func LogRequest() autorest.PrepareDecorator {
	return func(p autorest.Preparer) autorest.Preparer {
		return autorest.PreparerFunc(func(r *http.Request) (*http.Request, error) {
			r, err := p.Prepare(r)
			if err != nil {
				fmt.Println(err)
			}
			dump, _ := httputil.DumpRequestOut(r, true)
			fmt.Println(string(dump))
			return r, err
		})
	}
}

// LogResponse logs full autorest responses for any Azure client.
func LogResponse() autorest.RespondDecorator {
	return func(p autorest.Responder) autorest.Responder {
		return autorest.ResponderFunc(func(r *http.Response) error {
			err := p.Respond(r)
			if err != nil {
				fmt.Println(err)
			}
			dump, _ := httputil.DumpResponse(r, true)
			fmt.Println(string(dump))
			return err
		})
	}
}
