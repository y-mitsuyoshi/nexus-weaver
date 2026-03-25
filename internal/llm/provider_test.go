package llm

import "testing"

// コンパイル時のインターフェース準拠チェック
var _ LLMProvider = (*GeminiCLI)(nil)
var _ LLMProvider = (*LocalQwen)(nil)

func TestGetProvider_GeminiCLI(t *testing.T) {
	provider, err := GetProvider("gemini-cli")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, ok := provider.(*GeminiCLI); !ok {
		t.Errorf("expected *GeminiCLI, got %T", provider)
	}
}

func TestGetProvider_LocalQwen(t *testing.T) {
	provider, err := GetProvider("local-qwen")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	qwen, ok := provider.(*LocalQwen)
	if !ok {
		t.Errorf("expected *LocalQwen, got %T", provider)
	}
	if qwen.Endpoint == "" {
		t.Error("expected non-empty Endpoint")
	}
}

func TestGetProvider_Unknown(t *testing.T) {
	_, err := GetProvider("unknown-model")
	if err == nil {
		t.Fatal("expected error for unknown provider, got nil")
	}
}
