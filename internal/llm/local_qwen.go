package llm

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
)

// LocalQwen はコスト・速度重視のローカル Qwen モデル（OpenAI互換API）を呼び出すプロバイダです。
type LocalQwen struct {
	Endpoint string // e.g. "http://localhost:11434/v1/chat/completions"
	Model    string // e.g. "qwen3-30b-a3b" — 空の場合は "qwen2.5-coder" を使用
}

// chatRequest は OpenAI 互換 API へのリクエストボディです。
type chatRequest struct {
	Model    string        `json:"model"`
	Messages []chatMessage `json:"messages"`
}

// chatMessage は OpenAI 互換 API のメッセージ形式です。
type chatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// chatResponse は OpenAI 互換 API からのレスポンスボディです。
type chatResponse struct {
	Choices []struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
	} `json:"choices"`
}

// Generate はシステムプロンプトとユーザープロンプトを OpenAI 互換 API に送信し、
// 応答テキストを返します。
func (q *LocalQwen) Generate(systemPrompt, userPrompt string) (string, error) {
	modelName := q.Model
	if modelName == "" {
		modelName = "qwen2.5-coder"
	}

	reqBody := chatRequest{
		Model: modelName,
		Messages: []chatMessage{
			{Role: "system", Content: systemPrompt},
			{Role: "user", Content: userPrompt},
		},
	}

	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		return "", fmt.Errorf("failed to marshal request: %w", err)
	}

	resp, err := http.Post(q.Endpoint, "application/json", bytes.NewBuffer(jsonData))
	if err != nil {
		return "", fmt.Errorf("failed to send request to local Qwen: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("local Qwen returned status %d: %s", resp.StatusCode, string(body))
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read response body: %w", err)
	}

	var chatResp chatResponse
	if err := json.Unmarshal(body, &chatResp); err != nil {
		return "", fmt.Errorf("failed to parse response JSON: %w", err)
	}

	if len(chatResp.Choices) == 0 {
		return "", fmt.Errorf("no choices returned from local Qwen")
	}

	return strings.TrimSpace(chatResp.Choices[0].Message.Content), nil
}
