variable "aws_region" {
  description = "AWS region to deploy resources"
  type        = string
  default     = "us-east-1"
}

variable "github_telegram_bot_token" {
  description = "GitHub Telegram Bot Token from BotFather"
  type        = string
  sensitive   = true
}

variable "gitlab_telegram_bot_token" {
  description = "GitLab Telegram Bot Token from BotFather"
  type        = string
  sensitive   = true
}

variable "sentry_telegram_bot_token" {
  description = "Sentry Telegram Bot Token from BotFather (optional, the Sentry bot is disabled without it)"
  type        = string
  default     = ""
  sensitive   = true
}

variable "custom_domain" {
  description = "Custom domain name for the bot (optional)"
  type        = string
  default     = ""
}
