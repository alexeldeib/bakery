package main

import (
	"errors"

	"github.com/Azure/go-autorest/autorest"
	"github.com/Azure/go-autorest/autorest/azure"
)

const codeResourceGroupNotFound = "ResourceGroupNotFound"

// ResourceGroupNotFound parses the error to check if it's a resource group not found error.
func ResourceGroupNotFound(err error) bool {
	derr := autorest.DetailedError{}
	serr := &azure.ServiceError{}
	return errors.As(err, &derr) && errors.As(derr.Original, &serr) && serr.Code == codeResourceGroupNotFound
}

// ResourceNotFound parses the error to check if it's a resource not found error.
func ResourceNotFound(err error) bool {
	derr := autorest.DetailedError{}
	return errors.As(err, &derr) && derr.StatusCode == 404
}

func NotFound(err error) bool {
	return ResourceGroupNotFound(err) || ResourceNotFound(err)
}
