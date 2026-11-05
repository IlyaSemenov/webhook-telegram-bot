# Deploying to AWS Lambda with Terraform

This guide explains how to deploy the Webhook-Telegram Bot to AWS Lambda using Terraform.

> Heads up! AWS Lambda connectivity to Telegram servers turned to be unstable.
> The reference bots now run on a self-hosted Docker server.

## Prerequisites

1. [Terraform](https://www.terraform.io/downloads.html) installed (v1.0.0+)
2. AWS credentials configured (`~/.aws/credentials` or environment variables)
3. A Telegram bot token (from [@BotFather](https://t.me/BotFather))
4. Go installed on your local machine (1.25+)

## Step 1: Configure Terraform Variables

Create a `terraform.tfvars` file in the `terraform` directory based on the `terraform.tfvars.example` file.

## Step 2: Initialize and Apply Terraform

Initialize Terraform:

```bash
make terraform-init
```

Apply the configuration to create all required AWS resources:

```bash
make terraform-apply
```

When prompted, type `yes` to confirm. Terraform will create:

- IAM role and policies for Lambda
- Lambda function
- Lambda function URL
- CloudWatch Log Group
- All necessary permissions

After completion, Terraform will output:

- The Lambda function URL
- The Lambda function name
- The CloudWatch log group

## Step 3: Build and Deploy the Lambda Function

First, ensure all dependencies are properly downloaded:

```bash
go mod tidy
```

Build and deploy the function using the Makefile:

```bash
make update
```

This command will:

1. Build the Lambda function
2. Update the Lambda function code
3. Initialize the bot by calling the `/init` endpoint to set up the Telegram webhook

## Step 4: Test the Bot

1. Add your bot to a Telegram group
2. Use the `/start` command to initialize the bot
3. Use `/webhook` to get your webhook URL
4. Add this URL to your repository's webhook settings

## Updating the Bot

### Manual Updates

To manually update the bot after making changes:

```bash
make update
```

### Automatic Updates with GitHub Actions

The repository is configured with GitHub Actions to automatically deploy your bot whenever you push to the main branch.

#### Step 1: Add GitHub Secrets

In your GitHub repository:

1. Go to Settings → Secrets and variables → Actions
2. Add the following secrets:
   - `AWS_ACCESS_KEY_ID`: Your AWS access key
   - `AWS_SECRET_ACCESS_KEY`: Your AWS secret key
   - `AWS_REGION`: Your AWS region (e.g., `us-east-1`)

#### Step 2: Automatic Deployment

The GitHub workflow file (`.github/workflows/deploy.yml`) is already included in the repository.

Now that you've configured the AWS credentials in GitHub Secrets, automatic deployments are active. Every time you push to the main branch, GitHub Actions will automatically build and deploy your bot to AWS Lambda.

## Modifying Infrastructure

If you need to make changes to the infrastructure:

1. Edit the Terraform files in the `terraform` directory
2. Run `make terraform-apply` to apply the changes

## Destroying Resources

To remove all AWS resources created by Terraform:

```bash
make terraform-destroy
```

## Troubleshooting

If you encounter issues with the bot, you can check the CloudWatch logs:

```bash
make logs
```

You can also view logs in the AWS Console under CloudWatch → Log Groups.

## Using Custom Domain Name

To use a custom domain name, set it in the `terraform.tfvars` file.

Initially, running `make terraform-apply` will fail due to missing SSL certificate:

```log
InvalidViewerCertificate: The specified SSL certificate doesn't exist, isn't in the us-east-1 region, isn't valid, or doesn't include a valid certificate chain.

Check the certificate details:

```bash
CERTIFICATE_ARN=$(aws acm list-certificates --region us-east-1 \
  --query "CertificateSummaryList[?DomainName=='example.com'].CertificateArn" \
  --output text)
aws acm describe-certificate \
  --certificate-arn $CERTIFICATE_ARN \
  --query "Certificate.DomainValidationOptions" \
  --output json
```

Under `ResourceRecord`, you will see the DNS record that needs to be created.

Once the certificate validation succeeds, proceed with `make terraform-apply`.

Once the bot is deployed, you can update the DNS record to point to the CloudFront distribution's domain name (in the Terraform output `cloudfront_domain_name`).

## Publishing Multiple Versions

You can publish multiple versions of your bot using different Terraform workspaces.

For example, to deploy a staging version of your bot:

```bash
make deploy ENV=staging
```

This will create a new Terraform workspace for the staging environment and deploy the bot there.

Similarly, you can check the logs for the staging environment:

```bash
make logs ENV=staging
```

Make sure to use different Telegram bot tokens for multiple versions!
