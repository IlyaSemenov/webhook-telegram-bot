package telegram

// MessageSender defines the interface for sending Telegram messages.
type MessageSender interface {
	SendMessage(chatID int64, text string) error
}
