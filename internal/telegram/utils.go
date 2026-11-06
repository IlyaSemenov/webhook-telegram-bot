package telegram

import (
	"strconv"
)

func ParseChatID(chatIDStr string) (int64, error) {
	return strconv.ParseInt(chatIDStr, 10, 64)
}
