// Package main runs the audit service.
package main

import "github.com/aminio9/gereh/platform/go/service"

var version = "dev"

func main() {
	service.Run(service.Config{
		Name:           "audit",
		Version:        version,
		DefaultAddress: ":8094",
	}, nil)
}
