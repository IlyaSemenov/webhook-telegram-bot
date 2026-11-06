package telegram

import (
	"errors"
	"testing"
)

func TestIsBotBlockedError(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{
			name: "nil error",
			err:  nil,
			want: false,
		},
		{
			name: "bot was blocked by the user",
			err:  errors.New("Forbidden: bot was blocked by the user"),
			want: true,
		},
		{
			name: "user is deactivated",
			err:  errors.New("Forbidden: user is deactivated"),
			want: true,
		},
		{
			name: "chat not found",
			err:  errors.New("Bad Request: chat not found"),
			want: true,
		},
		{
			name: "bot is not a member",
			err:  errors.New("Forbidden: bot is not a member of the supergroup chat"),
			want: true,
		},
		{
			name: "unrelated error",
			err:  errors.New("timeout: connection reset"),
			want: false,
		},
		{
			name: "partial match should not trigger",
			err:  errors.New("the bot was not blocked"),
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isBotBlockedError(tt.err)
			if got != tt.want {
				t.Errorf("isBotBlockedError(%v) = %v, want %v", tt.err, got, tt.want)
			}
		})
	}
}
