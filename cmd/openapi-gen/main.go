package main

import (
	"encoding/json"
	"log"
	"os"

	"github.com/grantsy/grantsy/internal/entitlements"
	"github.com/grantsy/grantsy/internal/openapi"
	"github.com/grantsy/grantsy/internal/subscriptions"
)

func main() {
	reflector := openapi.NewReflector()

	// Register all API schemas
	entitlements.RegisterCheckSchema(reflector)
	entitlements.RegisterFeaturesSchema(reflector)
	entitlements.RegisterPlansSchema(reflector)
	subscriptions.RegisterSubscriptionSchema(reflector)
	// webhook intentionally excluded from OpenAPI documentation

	data, err := json.MarshalIndent(reflector.Spec, "", "  ")
	if err != nil {
		log.Fatal(err)
	}

	if err := os.WriteFile("openapi.json", data, 0644); err != nil {
		log.Fatal(err)
	}

	log.Println("Generated openapi.json")
}
