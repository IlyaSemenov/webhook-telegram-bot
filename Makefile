.PHONY: build clean deploy deploy-staging lint build-lambda update update-staging update-env run dev terraform-init terraform-apply terraform-destroy test test-update

# Environment variable defaults
ENV ?= prod
FUNCTION_NAME = $(if $(filter prod,$(ENV)),webhook-telegram-bot,webhook-telegram-bot-$(ENV))
TF_WORKSPACE="$(if $(filter prod,$(ENV)),default,$(ENV))"

# Build the application
build:
	go build -o bin/bot ./cmd/bot

# Clean build artifacts
clean:
	rm -rf ./bin

# Build for AWS Lambda
build-lambda:
	# For CGO, see https://github.com/aws/aws-lambda-go/issues/340
	GOOS=linux GOARCH=arm64 CGO_ENABLED=0 go build -o bin/bootstrap ./cmd/bot
	cd bin && zip function.zip bootstrap

# Initialize Terraform
terraform-init:
	cd terraform && terraform init

# Apply Terraform configuration
terraform-apply:
	cd terraform && TF_WORKSPACE=$(TF_WORKSPACE) terraform apply -var-file=$(TF_WORKSPACE).tfvars

# Destroy Terraform resources
terraform-destroy:
	cd terraform && TF_WORKSPACE=$(TF_WORKSPACE) terraform destroy -var-file=$(TF_WORKSPACE).tfvars

# Deploy to AWS Lambda using Terraform
deploy: build-lambda terraform-apply

# Update existing Lambda function
update: build-lambda
	aws lambda update-function-code \
		--function-name $(FUNCTION_NAME) \
		--zip-file fileb://bin/function.zip \
		--architectures arm64
	# Get the Lambda function URL and secret key from AWS
	LAMBDA_URL=$$(aws lambda get-function-url-config --function-name $(FUNCTION_NAME) --query "FunctionUrl" --output text) && \
	SECRET_KEY=$$(aws lambda get-function --function-name $(FUNCTION_NAME) --query "Configuration.Environment.Variables.SECRET_KEY" --output text) && \
	LAMBDA_INIT_URL=$${LAMBDA_URL}init && \
	echo "Initializing bot at $$LAMBDA_INIT_URL" && \
	curl -s -H "secret-key: $$SECRET_KEY" "$$LAMBDA_INIT_URL"

logs:
	aws logs tail /aws/lambda/$(FUNCTION_NAME) --follow

# Run linter
lint:
	go run github.com/golangci/golangci-lint/cmd/golangci-lint@latest run ./...
	gofmt -s -w .

# Run locally
run:
	go run ./cmd/bot

# Run with hot reload
dev:
	air -c .air.toml

# Run tests
test:
	go test ./...

# Update golden files
test-update:
	GOLDIE_UPDATE=1 go test -count=1 ./...
