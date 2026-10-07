package main

import (
	"context"
	"flag"
	"log"

	"github.com/aminparsa18/terraform-provider-flare/internal/provider"
	"github.com/hashicorp/terraform-plugin-framework/providerserver"
)

// version is set by the release build (-ldflags "-X main.version=...").
var version = "dev"

func main() {
	var debug bool
	flag.BoolVar(&debug, "debug", false, "run with support for debuggers like delve")
	flag.Parse()

	err := providerserver.Serve(context.Background(), provider.New(version), providerserver.ServeOpts{
		// Both registries resolve the same source address form; tofu uses registry.opentofu.org.
		Address: "registry.terraform.io/aminparsa18/flare",
		Debug:   debug,
	})
	if err != nil {
		log.Fatal(err.Error())
	}
}
