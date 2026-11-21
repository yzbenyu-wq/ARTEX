package agent

import "testing"

func TestChatToolOutputChars(t *testing.T) {
	tests := []struct {
		agentKey string
		want     int
	}{
		{"reporter", 12000},
		{"worker", 0},
		{"planner", 0},
		{"", 0},
	}
	for _, tt := range tests {
		t.Run(tt.agentKey, func(t *testing.T) {
			if got := chatToolOutputChars(tt.agentKey); got != tt.want {
				t.Fatalf("chatToolOutputChars(%q) = %d, want %d", tt.agentKey, got, tt.want)
			}
		})
	}
}
