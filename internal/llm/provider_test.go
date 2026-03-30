package llm

import "testing"

// コンパイル時のインターフェース準拠チェック
var _ LLMProvider = (*GeminiCLI)(nil)
var _ LLMProvider = (*CopilotCLI)(nil)
var _ LLMProvider = (*LocalQwen)(nil)

func TestParseModelSpec(t *testing.T) {
	tests := []struct {
		spec         string
		wantProvider string
		wantModel    string
	}{
		{"gemini-cli", "gemini-cli", ""},
		{"gemini-cli:gemini-2.5-flash", "gemini-cli", "gemini-2.5-flash"},
		{"copilot-cli:claude-opus-4", "copilot-cli", "claude-opus-4"},
		{"local-qwen", "local-qwen", ""},
		{"local-qwen:qwen3-30b-a3b", "local-qwen", "qwen3-30b-a3b"},
		{"gemini-cli:default", "gemini-cli", ""},
		{"copilot-cli:default", "copilot-cli", ""},
	}
	for _, tc := range tests {
		t.Run(tc.spec, func(t *testing.T) {
			p, m := parseModelSpec(tc.spec)
			if p != tc.wantProvider {
				t.Errorf("provider: got %q, want %q", p, tc.wantProvider)
			}
			if m != tc.wantModel {
				t.Errorf("model: got %q, want %q", m, tc.wantModel)
			}
		})
	}
}

func TestGetProvider_GeminiCLI(t *testing.T) {
	tests := []struct {
		name      string
		spec      string
		wantModel string
	}{
		{"Default", "gemini-cli", ""},
		{"WithModel", "gemini-cli:gemini-2.5-flash", "gemini-2.5-flash"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			provider, err := GetProvider(tc.spec)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			g, ok := provider.(*GeminiCLI)
			if !ok {
				t.Fatalf("expected *GeminiCLI, got %T", provider)
			}
			if g.Model != tc.wantModel {
				t.Errorf("model: got %q, want %q", g.Model, tc.wantModel)
			}
		})
	}
}

func TestGetProvider_CopilotCLI(t *testing.T) {
	tests := []struct {
		name      string
		spec      string
		wantModel string
	}{
		{"Default", "copilot-cli", ""},
		{"ClaudeOpus", "copilot-cli:claude-opus-4", "claude-opus-4"},
		{"GPT4o", "copilot-cli:gpt-4o", "gpt-4o"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			provider, err := GetProvider(tc.spec)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			c, ok := provider.(*CopilotCLI)
			if !ok {
				t.Fatalf("expected *CopilotCLI, got %T", provider)
			}
			if c.Model != tc.wantModel {
				t.Errorf("model: got %q, want %q", c.Model, tc.wantModel)
			}
		})
	}
}

func TestGetProvider_LocalQwen(t *testing.T) {
	tests := []struct {
		name      string
		spec      string
		wantModel string
	}{
		{"Default", "local-qwen", ""},
		{"WithModel", "local-qwen:qwen3-30b-a3b", "qwen3-30b-a3b"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			provider, err := GetProvider(tc.spec)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			qwen, ok := provider.(*LocalQwen)
			if !ok {
				t.Fatalf("expected *LocalQwen, got %T", provider)
			}
			if qwen.Endpoint == "" {
				t.Error("expected non-empty Endpoint")
			}
			if qwen.Model != tc.wantModel {
				t.Errorf("model: got %q, want %q", qwen.Model, tc.wantModel)
			}
		})
	}
}

func TestGetProvider_Unknown(t *testing.T) {
	_, err := GetProvider("unknown-model")
	if err == nil {
		t.Fatal("expected error for unknown provider, got nil")
	}
}

func TestCleanCopilotOutput(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "plain text",
			input: "improvement/dry-run-step-summary",
			want:  "improvement/dry-run-step-summary",
		},
		{
			name: "shell execution block",
			input: "● Output branch name (shell)\n" +
				"  │ echo \"improvement/dry-run-step-summary\"\n" +
				"  └ 2 lines...\n" +
				"\n" +
				"improvement/dry-run-step-summary",
			want: "improvement/dry-run-step-summary",
		},
		{
			name: "multiple blocks",
			input: "● Run command (shell)\n" +
				"  │ ls -la\n" +
				"  └ 5 lines...\n" +
				"\n" +
				"result line 1\n" +
				"result line 2",
			want: "result line 1\nresult line 2",
		},
		{
			name:  "empty input",
			input: "",
			want:  "",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := cleanCopilotOutput(tc.input)
			if got != tc.want {
				t.Errorf("got %q, want %q", got, tc.want)
			}
		})
	}
}
