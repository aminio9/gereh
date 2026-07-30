// Package main runs the runtime-manager service.
package main

import "github.com/aminio9/gereh/platform/go/service"

var version = "dev"

func main() {
	service.Run(service.Config{
		Name:           "runtime-manager",
		Version:        version,
		DefaultAddress: ":8089",
	}, nil)
}
