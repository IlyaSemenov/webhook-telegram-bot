output "lambda_function_url" {
  description = "URL of the Lambda function (or CloudFront URL if custom domain is used)"
  value       = local.use_custom_domain ? "https://${var.custom_domain}" : aws_lambda_function_url.webhook_telegram_bot_url.function_url
}

output "cloudwatch_log_group" {
  description = "CloudWatch Log Group for Lambda logs"
  value       = aws_cloudwatch_log_group.lambda_logs.name
}

output "secret_key" {
  description = "Generated secret key (sensitive)"
  value       = random_password.secret_key.result
  sensitive   = true
}

output "github_actions_access_key" {
  description = "AWS Access Key ID for GitHub Actions"
  value       = terraform.workspace == "default" ? aws_iam_access_key.github_actions[0].id : "Not created for ${terraform.workspace}"
}

output "github_actions_secret_key" {
  description = "AWS Secret Access Key for GitHub Actions (sensitive)"
  value       = terraform.workspace == "default" ? aws_iam_access_key.github_actions[0].secret : "Not created for ${terraform.workspace}"
  sensitive   = true
}

# ACM Certificate validation records (only when custom domain is used)
output "acm_certificate_validation_records" {
  description = "DNS records to add to your DNS provider for ACM certificate validation"
  value = local.use_custom_domain ? {
    for dvo in aws_acm_certificate.cert[0].domain_validation_options : dvo.domain_name => {
      name  = dvo.resource_record_name
      type  = dvo.resource_record_type
      value = dvo.resource_record_value
    }
  } : {}
}

output "cloudfront_domain_name" {
  description = "CloudFront domain name (only when custom domain is used)"
  value       = local.use_custom_domain ? aws_cloudfront_distribution.distribution[0].domain_name : null
}
