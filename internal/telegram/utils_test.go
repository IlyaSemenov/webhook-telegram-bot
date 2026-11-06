package telegram

import (
	"testing"
)

func TestParseChatID(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    int64
		wantErr bool
	}{
		{
			name:  "valid positive chat ID",
			input: "123456789",
			want:  123456789,
		},
		{
			name:  "valid negative chat ID (group)",
			input: "-1001234567890",
			want:  -1001234567890,
		},
		{
			name:  "zero",
			input: "0",
			want:  0,
		},
		{
			name:    "empty string",
			input:   "",
			wantErr: true,
		},
		{
			name:    "non-numeric",
			input:   "abc",
			wantErr: true,
		},
		{
			name:    "float-like string",
			input:   "123.456",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseChatID(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseChatID(%q) error = %v, wantErr %v", tt.input, err, tt.wantErr)
				return
			}
			if !tt.wantErr && got != tt.want {
				t.Errorf("ParseChatID(%q) = %d, want %d", tt.input, got, tt.want)
			}
		})
	}
}
