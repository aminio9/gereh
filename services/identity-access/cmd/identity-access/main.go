// Package main runs the identity-access service.
package main

import "github.com/aminio9/gereh/platform/go/service"

var version = "dev"

func main() {
	service.Run(service.Config{
		Name:           "identity-access",
		Version:        version,
		DefaultAddress: ":8081",
	}, nil)
}
