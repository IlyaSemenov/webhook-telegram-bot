# ACM Certificate for custom domain (only created when using custom domain)
resource "aws_acm_certificate" "cert" {
  count             = local.use_custom_domain ? 1 : 0
  provider          = aws.us_east_1
  domain_name       = var.custom_domain
  validation_method = "DNS"

  lifecycle {
    create_before_destroy = true
  }
}

resource "aws_cloudfront_origin_request_policy" "lambda_webhook_headers" {
  name    = "${local.function_name}-webhook-headers-policy"
  comment = "Forwards webhook headers to Lambda"

  cookies_config {
    cookie_behavior = "none"
  }

  headers_config {
    header_behavior = "whitelist"
    headers {
      items = [
        "x-gitlab-event",
        "x-github-event",
        "x-telegram-bot-api-secret-token",
        "content-type",
        "user-agent",
        "secret-key",
      ]
    }
  }

  query_strings_config {
    query_string_behavior = "all" # Forward all query params (if needed)
  }
}

# CloudFront distribution for custom domain
resource "aws_cloudfront_distribution" "distribution" {
  count = local.use_custom_domain ? 1 : 0

  origin {
    # Use the Lambda function URL as the origin
    domain_name = trimsuffix(trimprefix(aws_lambda_function_url.webhook_telegram_bot_url.function_url, "https://"), "/")
    origin_id   = "LambdaOrigin"

    custom_origin_config {
      http_port              = 80
      https_port             = 443
      origin_protocol_policy = "https-only"
      origin_ssl_protocols   = ["TLSv1.2"]
    }
  }

  enabled             = true
  is_ipv6_enabled     = true
  default_root_object = ""

  aliases = [var.custom_domain]

  default_cache_behavior {
    allowed_methods        = ["DELETE", "GET", "HEAD", "OPTIONS", "PATCH", "POST", "PUT"]
    cached_methods         = ["GET", "HEAD"]
    target_origin_id       = "LambdaOrigin"
    viewer_protocol_policy = "redirect-to-https"

    cache_policy_id          = "4135ea2d-6df8-44a3-9df3-4b5a84be39ad" # CachingDisabled
    origin_request_policy_id = aws_cloudfront_origin_request_policy.lambda_webhook_headers.id
  }

  price_class = "PriceClass_100"

  restrictions {
    geo_restriction {
      restriction_type = "none"
    }
  }

  viewer_certificate {
    acm_certificate_arn      = aws_acm_certificate.cert[0].arn
    ssl_support_method       = "sni-only"
    minimum_protocol_version = "TLSv1.2_2021"
  }
}
