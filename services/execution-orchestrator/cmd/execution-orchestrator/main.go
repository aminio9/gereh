package main

import "github.com/aminio9/gereh-platform/platform/go/service"

var version = "dev"

func main() {
	service.Run(service.Config{
		Name:           "execution-orchestrator",
		Version:        version,
		DefaultAddress: ":8088",
	}, nil)
}
