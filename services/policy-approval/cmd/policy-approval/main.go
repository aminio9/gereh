// Package main runs the policy-approval service.
package main

import "github.com/aminio9/gereh/platform/go/service"

var version = "dev"

func main() {
	service.Run(service.Config{
		Name:           "policy-approval",
		Version:        version,
		DefaultAddress: ":8085",
	}, nil)
}
