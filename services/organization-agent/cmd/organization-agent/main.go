// Package main runs the organization-agent service.
package main

import "github.com/aminio9/gereh/platform/go/service"

var version = "dev"

func main() {
	service.Run(service.Config{
		Name:           "organization-agent",
		Version:        version,
		DefaultAddress: ":8083",
	}, nil)
}
