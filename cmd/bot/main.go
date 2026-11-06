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
	cfg        *config.Config
	srv        *server.Server
	muxAdapter *muxadapter.GorillaMuxAdapterV2
)

func init() {
	var err error
	cfg, err = config.Load()
	if err != nil {
		log.Fatalf("Failed to load configuration: %v", err)
	}

	// Create server
	srv, err = server.New(cfg)
	if err != nil {
		log.Fatalf("Failed to create server: %v", err)
	}

	// Create Lambda adapter if in Lambda mode
	if cfg.IsLambda {
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
	if cfg.IsLambda {
		// Start Lambda handler
		lambda.Start(lambdaHandler)
	} else {
		// For local development, the server is already started in init()
		// Just block forever
		select {}
	}
}
