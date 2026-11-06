package config

import (
	"context"
	"fmt"
	"log"
	"net/url"
	"os"
	"strings"

	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/lambda"
	"github.com/joho/godotenv"
)

// Config holds the deployment settings read from the environment.
type Config struct {
	ListenAddress               string
	SecretKey                   string
	BaseURL                     string
	IsLambda                    bool
	StorageConnectionStringBase string
}

func GetLambdaURL() (string, error) {
	ctx := context.Background()

	// Load AWS config (uses Lambda's built-in IAM role)
	cfg, err := config.LoadDefaultConfig(ctx)
	if err != nil {
		return "", fmt.Errorf("Failed to load AWS config: %v", err)
	}

	// Create Lambda client
	client := lambda.NewFromConfig(cfg)

	// Get function URL config
	functionName := os.Getenv("AWS_LAMBDA_FUNCTION_NAME")
	resp, err := client.GetFunctionUrlConfig(ctx, &lambda.GetFunctionUrlConfigInput{
		FunctionName: &functionName,
	})
	if err != nil {
		return "", fmt.Errorf("Failed to get function URL: %v", err)
	}

	return *resp.FunctionUrl, nil
}

// Load reads the configuration from the environment and the optional .env file.
func Load() (*Config, error) {
	var lambdaFunctionName = os.Getenv("AWS_LAMBDA_FUNCTION_NAME")

	// Check if we're running in Lambda
	isLambda := lambdaFunctionName != ""

	// Load .env file if it exists and we're not in Lambda
	if !isLambda {
		if err := godotenv.Load(); err != nil {
			log.Printf("Warning: Error loading .env file: %v", err)
		}
	}

	var listenAddress string
	if listenPort := os.Getenv("PORT"); listenPort != "" {
		listenAddress = ":" + listenPort
	} else {
		listenAddress = ":8080"
	}

	// Try to auto-discover BASE_URL if we're in Lambda
	baseURL := os.Getenv("BASE_URL")
	if isLambda && baseURL == "" {
		var err error
		baseURL, err = GetLambdaURL()
		if err != nil {
			return nil, err
		}
		fmt.Printf("Auto-discovered BASE_URL: %s\n", baseURL)
	}

	// Ensure BaseURL doesn't end with a slash
	baseURL = strings.TrimSuffix(baseURL, "/")

	secretKey := os.Getenv("SECRET_KEY")
	if secretKey == "" {
		return nil, fmt.Errorf("SECRET_KEY environment variable is missing")
	}

	// Auto-detect storage connection string base
	var storageConnectionStringBase string
	if mongoURL := os.Getenv("MONGO_URL"); mongoURL != "" {
		// Running with MongoDB
		storageConnectionStringBase = setupMongoEnvAndGetConnectionStringBase(mongoURL)
	} else if isLambda {
		// Running in AWS - use DynamoDB
		storageConnectionStringBase = fmt.Sprintf("dynamodb://%s", lambdaFunctionName)
	}

	return &Config{
		ListenAddress:               listenAddress,
		SecretKey:                   secretKey,
		BaseURL:                     baseURL,
		IsLambda:                    isLambda,
		StorageConnectionStringBase: storageConnectionStringBase,
	}, nil
}

// Due to MongoDB driver requirement, we need to set the MONGO_SERVER_URL env var.
func setupMongoEnvAndGetConnectionStringBase(mongoURL string) string {
	// Parse the URL to extract components.
	u, err := url.Parse(mongoURL)
	if err != nil {
		log.Fatalf("Failed to parse MONGO_URL: %v", err)
	}

	// Extract db name from path (trim leading /).
	dbName := strings.TrimPrefix(u.Path, "/")
	if dbName == "" {
		log.Fatal("MONGO_URL missing database name")
	}

	// Set the env var for the docstore mongo opener.
	if err := os.Setenv("MONGO_SERVER_URL", mongoURL); err != nil {
		log.Fatalf("Failed to set MONGO_SERVER_URL: %v", err)
	}

	return fmt.Sprintf("mongo://%s", dbName)
}
