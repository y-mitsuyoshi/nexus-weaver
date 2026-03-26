package llm

import (
	"fmt"
	"os"
)

// LLMProvider はエージェントの実装を差し替え可能にするための共通インターフェースです。
// システムプロンプトとユーザー入力を受け取り、LLMの応答テキストを返します。
type LLMProvider interface {
	Generate(systemPrompt, userPrompt string) (string, error)
}

// GetProvider はモデル名に基づいて適切な LLMProvider 実装を返すファクトリ関数です。
func GetProvider(modelName string) (LLMProvider, error) {
	switch modelName {
	case "gemini-cli":
		return &GeminiCLI{}, nil
	case "copilot-cli", "gpt-4", "gpt-4o", "gpt-5":
		return &CopilotCLI{
			Model: modelName,
		}, nil
	case "local-qwen":
		endpoint := os.Getenv("LOCAL_QWEN_ENDPOINT")
		if endpoint == "" {
			endpoint = "http://localhost:11434/v1/chat/completions"
		}
		return &LocalQwen{
			Endpoint: endpoint,
		}, nil
	default:
		return nil, fmt.Errorf("unknown LLM provider: %s", modelName)
	}
}
