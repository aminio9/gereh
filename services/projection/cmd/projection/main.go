package main

import "github.com/aminio9/gereh-platform/platform/go/service"

var version = "dev"

func main() {
	service.Run(service.Config{
		Name:           "projection",
		Version:        version,
		DefaultAddress: ":8093",
	}, nil)
}
