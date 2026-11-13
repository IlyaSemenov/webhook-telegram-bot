package telegram

import (
	"context"
	"fmt"
	"html"
	"log"

	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
)

// webhookURLNotice is the advice that the /webhook command shows with the URL.
const webhookURLNotice = "Keep this URL private: anyone who has it can send messages to this chat. " +
	"If it leaks, reissue it with the button below."

// webhookURLErrorText is the reply of the /webhook command when the URL cannot be issued.
const webhookURLErrorText = "❌ Failed to get the webhook URL. Please try again later."

// reissueWarningText is the alert that asks to confirm reissuing the webhook URL.
const reissueWarningText = "All previous webhook URLs of this bot in this chat will stop working. Confirm to reissue the URL."

// reissueErrorText is the alert shown when the webhook URL cannot be reissued.
const reissueErrorText = "❌ Failed to reissue the webhook URL. Please try again later."

// reissuedText is the beginning of the /webhook reply with a reissued URL.
const reissuedText = "✅ <b>The webhook URL has been reissued.</b> All previous URLs of this bot in this chat no longer work: " +
	"replace them with the new one in every webhook."

// Callback data of the buttons under the /webhook reply.
const (
	reissueData        = "webhook:reissue"
	confirmReissueData = "webhook:reissue:confirm"
	cancelReissueData  = "webhook:reissue:cancel"
)

// reissueKeyboard is the button under the /webhook reply that starts reissuing the URL.
var reissueKeyboard = &models.InlineKeyboardMarkup{
	InlineKeyboard: [][]models.InlineKeyboardButton{
		{{Text: "🔄 Reissue URL", CallbackData: reissueData}},
	},
}

// confirmReissueKeyboard is the keyboard with the buttons under the /webhook reply that confirm or cancel reissuing the URL.
var confirmReissueKeyboard = &models.InlineKeyboardMarkup{
	InlineKeyboard: [][]models.InlineKeyboardButton{
		{{Text: "⚠️ Revoke previous URLs", CallbackData: confirmReissueData}, {Text: "Cancel", CallbackData: cancelReissueData}},
	},
}

// webhookCommand is the /webhook command of a bot, which shows the webhook URL of the chat and reissues it.
type webhookCommand struct {
	service    *TelegramService
	sourceName string // Name of the source in the reply
	setupText  string // HTML instructions for adding the URL to the source
}

// RegisterWebhookCommand registers the /webhook command, which replies with the webhook URL of the chat
// followed by setupText, the HTML instructions for adding the URL to the source named sourceName.
// The button under the reply reissues the URL after a confirmation.
func (s *TelegramService) RegisterWebhookCommand(sourceName string, setupText string) {
	c := &webhookCommand{service: s, sourceName: sourceName, setupText: setupText}
	s.RegisterCommandHandler("webhook", func(ctx context.Context, _ *bot.Bot, update *models.Update) {
		c.sendWebhookURL(ctx, update.Message.Chat.ID)
	})
	s.registerCallbackHandler(reissueData, c.handleReissue)
	s.registerCallbackHandler(confirmReissueData, c.handleConfirmReissue)
	s.registerCallbackHandler(cancelReissueData, c.handleCancelReissue)
}

// formatText returns the reply with the webhook URL.
func (c *webhookCommand) formatText(webhookURL string) string {
	return fmt.Sprintf("🔗 <b>Your %s Webhook URL</b>\n\n<code>%s</code>\n\n", c.sourceName, html.EscapeString(webhookURL)) +
		webhookURLNotice + "\n\n" +
		c.setupText
}

// sendWebhookURL sends the webhook URL of the chat with the setup instructions, or an error message if the URL cannot be issued.
func (c *webhookCommand) sendWebhookURL(ctx context.Context, chatID int64) {
	s := c.service
	webhookURL, err := s.getChatWebhookURL(ctx, chatID)
	if err != nil {
		log.Printf("Failed to get %s webhook URL of chat %d: %v", s.botId, chatID, err)
		s.SendMessageOrLogError(chatID, webhookURLErrorText)
		return
	}

	if _, err := s.sendMessage(chatID, c.formatText(webhookURL), reissueKeyboard); err != nil {
		log.Printf("Failed to send %s webhook URL to chat %d: %v", s.botId, chatID, err)
	}
}

// handleReissue warns that reissuing the URL invalidates the previous ones and asks to confirm it.
func (c *webhookCommand) handleReissue(_ context.Context, query *models.CallbackQuery) {
	c.service.answerCallbackQuery(query.ID, reissueWarningText)
	c.updateKeyboard(query, confirmReissueKeyboard)
}

// handleCancelReissue restores the button that starts reissuing the URL.
func (c *webhookCommand) handleCancelReissue(_ context.Context, query *models.CallbackQuery) {
	c.service.answerCallbackQuery(query.ID, "")
	c.updateKeyboard(query, reissueKeyboard)
}

// handleConfirmReissue reissues the webhook URL of the chat and shows the new URL in place of the old one,
// or in a new message if the old one cannot be edited.
func (c *webhookCommand) handleConfirmReissue(ctx context.Context, query *models.CallbackQuery) {
	s := c.service
	chatID, messageID, ok := getCallbackMessage(query)
	if !ok {
		s.answerCallbackQuery(query.ID, "")
		return
	}

	issuedAt, err := s.chatStorage.ReissueWebhookURL(ctx, chatID, s.botId)
	if err != nil {
		log.Printf("Failed to reissue %s webhook URL of chat %d: %v", s.botId, chatID, err)
		s.answerCallbackQuery(query.ID, reissueErrorText)
		c.updateKeyboard(query, reissueKeyboard)
		return
	}
	s.answerCallbackQuery(query.ID, "")

	text := reissuedText + "\n\n" + c.formatText(s.createWebhookURL(chatID, issuedAt))
	if err := s.updateMessage(chatID, messageID, text, reissueKeyboard); err != nil {
		log.Printf("Failed to show the reissued %s webhook URL in message %d of chat %d, sending a new message: %v", s.botId, messageID, chatID, err)
		if _, err := s.sendMessage(chatID, text, reissueKeyboard); err != nil {
			log.Printf("Failed to send the reissued %s webhook URL to chat %d: %v", s.botId, chatID, err)
		}
	}
}

// updateKeyboard replaces the buttons of the message with the pressed button.
func (c *webhookCommand) updateKeyboard(query *models.CallbackQuery, keyboard *models.InlineKeyboardMarkup) {
	chatID, messageID, ok := getCallbackMessage(query)
	if !ok {
		return
	}
	if err := c.service.updateReplyMarkup(chatID, messageID, keyboard); err != nil {
		log.Printf("Failed to update the buttons of %s message %d in chat %d: %v", c.service.botId, messageID, chatID, err)
	}
}

// getCallbackMessage returns the chat and the message with the pressed button,
// or false for a button of a message sent in inline mode, which the bot does not use.
func getCallbackMessage(query *models.CallbackQuery) (chatID int64, messageID int, ok bool) {
	switch {
	case query.Message.Message != nil:
		return query.Message.Message.Chat.ID, query.Message.Message.ID, true
	case query.Message.InaccessibleMessage != nil:
		return query.Message.InaccessibleMessage.Chat.ID, query.Message.InaccessibleMessage.MessageID, true
	}
	return 0, 0, false
}
