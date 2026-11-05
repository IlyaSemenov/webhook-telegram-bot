package main

import (
	"context"
	"log"

	"webhook-telegram-bot/internal/config"
	"webhook-telegram-bot/internal/server"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
	muxadapter "github.com/awslabs/aws-lambda-go-api-proxy/gorillamux"
)

var (
	srv        *server.Server
	muxAdapter *muxadapter.GorillaMuxAdapterV2
)

func init() {
	// Initialize global configuration
	if err := config.Initialize(); err != nil {
		log.Fatalf("Failed to initialize configuration: %v", err)
	}

	// Create server
	var err error
	srv, err = server.New()
	if err != nil {
		log.Fatalf("Failed to create server: %v", err)
	}

	// Create Lambda adapter if in Lambda mode
	if config.Global.IsLambda {
		muxAdapter = muxadapter.NewV2(srv.Router())
	} else {
		// Start HTTP server for local development
		go func() {
			if err := srv.ListenAndServe(); err != nil {
				log.Fatalf("Server error: %v", err)
			}
		}()
	}
}

func lambdaHandler(ctx context.Context, req events.APIGatewayV2HTTPRequest) (events.APIGatewayV2HTTPResponse, error) {
	return muxAdapter.ProxyWithContext(ctx, req)
}

func main() {
	if config.Global.IsLambda {
		// Start Lambda handler
		lambda.Start(lambdaHandler)
	} else {
		// For local development, the server is already started in init()
		// Just block forever
		select {}
	}
}
