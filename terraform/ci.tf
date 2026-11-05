
# IAM User for GitHub Actions (only for production)
resource "aws_iam_user" "github_actions" {
  count = terraform.workspace == "default" ? 1 : 0
  name  = "github-actions-webhook-telegram-bot"
}

# IAM Policy for GitHub Actions (only for production)
resource "aws_iam_policy" "github_actions_policy" {
  count       = terraform.workspace == "default" ? 1 : 0
  name        = "github-actions-webhook-telegram-bot-policy"
  description = "Policy for GitHub Actions to update Lambda code"

  policy = jsonencode({
    Version = "2012-10-17"
    Statement = [
      {
        Effect = "Allow"
        Action = [
          "lambda:UpdateFunctionCode",
          "lambda:GetFunction",
          "lambda:GetFunctionUrlConfig"
        ]
        Resource = aws_lambda_function.webhook_telegram_bot.arn
      }
    ]
  })
}

# Attach policy to IAM user (only for production)
resource "aws_iam_user_policy_attachment" "github_actions_policy_attachment" {
  count      = terraform.workspace == "default" ? 1 : 0
  user       = aws_iam_user.github_actions[0].name
  policy_arn = aws_iam_policy.github_actions_policy[0].arn
}

# Access key for GitHub Actions (only for production)
resource "aws_iam_access_key" "github_actions" {
  count = terraform.workspace == "default" ? 1 : 0
  user  = aws_iam_user.github_actions[0].name
}
