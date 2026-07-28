package main

import "github.com/aminio9/gereh-platform/platform/go/service"

var version = "dev"

func main() {
	service.Run(service.Config{
		Name:           "model-gateway",
		Version:        version,
		DefaultAddress: ":8087",
	}, nil)
}
