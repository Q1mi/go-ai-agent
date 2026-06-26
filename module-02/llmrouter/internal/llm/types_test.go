package llm

import "testing"

func TestWithTemperatureKeepsExplicitZero(t *testing.T) {
	req := NewChatRequest(
		"",
		[]Message{{Role: RoleUser, Content: "hello"}},
		WithTemperature(0),
	)
	if req.Temperature == nil || *req.Temperature != 0 {
		t.Fatalf("Temperature = %v, want pointer to zero", req.Temperature)
	}
}

func TestValidateRequest(t *testing.T) {
	tests := []struct {
		name    string
		req     ChatRequest
		wantErr bool
	}{
		{
			name: "valid",
			req: ChatRequest{
				Messages: []Message{{Role: RoleUser, Content: "hello"}},
			},
		},
		{name: "empty messages", req: ChatRequest{}, wantErr: true},
		{
			name: "unknown role",
			req: ChatRequest{
				Messages: []Message{{Role: "unknown", Content: "hello"}},
			},
			wantErr: true,
		},
		{
			name: "empty content",
			req: ChatRequest{
				Messages: []Message{{Role: RoleUser, Content: " "}},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateRequest(tt.req)
			if (err != nil) != tt.wantErr {
				t.Fatalf("ValidateRequest() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
