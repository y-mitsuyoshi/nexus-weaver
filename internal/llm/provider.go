package llm

import (
	"fmt"
	"os"
	"strings"
)

// LLMProvider はエージェントの実装を差し替え可能にするための共通インターフェースです。
// システムプロンプトとユーザー入力を受け取り、LLMの応答テキストを返します。
type LLMProvider interface {
	Generate(systemPrompt, userPrompt string) (string, error)
}

// parseModelSpec は "provider:model" 形式の文字列をプロバイダー名とモデル名に分割します。
// ":" が含まれない場合はプロバイダー名のみを返し、モデル名は空になります。
// 例:
//   - "gemini-cli:gemini-2.5-flash" → ("gemini-cli", "gemini-2.5-flash")
//   - "copilot-cli:claude-opus-4"   → ("copilot-cli", "claude-opus-4")
//   - "gemini-cli"                  → ("gemini-cli", "")
//   - "local-qwen:qwen3-30b-a3b"   → ("local-qwen", "qwen3-30b-a3b")
func parseModelSpec(spec string) (provider string, model string) {
	if idx := strings.IndexByte(spec, ':'); idx >= 0 {
		provider = spec[:idx]
		model = spec[idx+1:]
		if model == "default" {
			model = ""
		}
		return provider, model
	}
	return spec, ""
}

// GetProvider はモデル指定文字列に基づいて適切な LLMProvider 実装を返すファクトリ関数です。
// "provider:model" 形式でプロバイダーと使用モデルを個別に指定できます。
// 例: "gemini-cli:gemini-2.5-flash", "copilot-cli:claude-opus-4", "local-qwen:qwen3-30b-a3b"
// ":" を省略した場合は従来通りプロバイダーのデフォルトモデルが使用されます。
func GetProvider(modelSpec string) (LLMProvider, error) {
	providerName, modelName := parseModelSpec(modelSpec)

	switch providerName {
	case "gemini-cli":
		return &GeminiCLI{Model: modelName}, nil
	case "copilot-cli":
		return &CopilotCLI{Model: modelName}, nil
	case "local-qwen":
		endpoint := os.Getenv("LOCAL_QWEN_ENDPOINT")
		if endpoint == "" {
			endpoint = "http://localhost:11434/v1/chat/completions"
		}
		return &LocalQwen{
			Endpoint: endpoint,
			Model:    modelName,
		}, nil
	default:
		return nil, fmt.Errorf("unknown LLM provider: %s (from spec %q)", providerName, modelSpec)
	}
}
