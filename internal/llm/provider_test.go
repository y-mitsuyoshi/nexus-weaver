package llm

import (
	"strings"
	"testing"
)

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
			name: "shell execution block with bullet",
			input: "● Output branch name (shell)\n" +
				"  │ echo \"improvement/dry-run-step-summary\"\n" +
				"  └ 2 lines...\n" +
				"\n" +
				"improvement/dry-run-step-summary",
			want: "improvement/dry-run-step-summary",
		},
		{
			name: "failed and success markers",
			input: "✗ Check existing test files (shell)\n" +
				"\n" +
				"✗ Find test files (shell)\n" +
				"\n" +
				"actual content here\n" +
				"\n" +
				"✓ Edit main.go\n" +
				"more actual content",
			want: "actual content here\n\nmore actual content",
		},
		{
			name: "tree formatting with branch markers",
			input: "● Run command (shell)\n" +
				"  ├ step 1\n" +
				"  │ ls -la\n" +
				"  └ 5 lines...\n" +
				"\n" +
				"result line",
			want: "result line",
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

func TestFormatPrompt_StructuredOutput(t *testing.T) {
	result := formatPrompt("you are a PM", "write a PRD")
	if strings.Contains(result, "<instructions>") {
		t.Error("should not contain <instructions> guard block")
	}
	if !strings.Contains(result, "<system>") {
		t.Error("expected <system> block")
	}
	if !strings.Contains(result, "you are a PM") {
		t.Error("expected system prompt content")
	}
	if !strings.Contains(result, "<user>") {
		t.Error("expected <user> block")
	}
	if !strings.Contains(result, "write a PRD") {
		t.Error("expected user prompt content")
	}
}

func TestFormatPrompt_NoSystemPrompt(t *testing.T) {
	result := formatPrompt("", "just a question")
	if strings.Contains(result, "<system>") {
		t.Error("should not contain <system> when systemPrompt is empty")
	}
	if !strings.Contains(result, "just a question") {
		t.Error("expected user prompt content")
	}
}
