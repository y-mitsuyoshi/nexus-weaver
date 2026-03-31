package llm

import (
	"bytes"
	"fmt"
	"os/exec"
	"strings"
)

// GeminiCLI は高推論用の Gemini モデルを CLI 経由で呼び出すプロバイダです。
// `gemini` コマンドを os/exec で実行し、標準出力を応答として返します。
type GeminiCLI struct {
	Model string // e.g. "gemini-2.5-flash", "gemini-2.5-pro" — 空の場合は gemini CLI のデフォルト
}

// Generate はシステムプロンプトとユーザープロンプトを結合して
// Gemini CLI にワンショット実行（-p オプション）で渡し、応答テキストを返します。
func (g *GeminiCLI) Generate(systemPrompt, userPrompt string) (string, error) {
	// gemini コマンドの存在確認
	if _, err := exec.LookPath("gemini"); err != nil {
		return "", fmt.Errorf("'gemini' command not found in PATH. Please install it first")
	}

	// プロンプトを構造化して結合
	combinedPrompt := formatPrompt(systemPrompt, userPrompt)

	// gemini コマンドを実行（-p でワンショット、--model でモデル指定）
	args := []string{"-p", combinedPrompt}
	if g.Model != "" {
		args = append(args, "--model", g.Model)
	}
	cmd := exec.Command("gemini", args...)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("gemini CLI execution failed: %w\nstderr: %s", err, stderr.String())
	}

	return strings.TrimSpace(stdout.String()), nil
}

// formatPrompt はシステムプロンプトとユーザープロンプトを
// 構造化されたテキストに結合します。
func formatPrompt(systemPrompt, userPrompt string) string {
	var sb strings.Builder
	if systemPrompt != "" {
		sb.WriteString("<system>\n")
		sb.WriteString(systemPrompt)
		sb.WriteString("\n</system>\n\n")
	}
	sb.WriteString("<user>\n")
	sb.WriteString(userPrompt)
	sb.WriteString("\n</user>")
	return sb.String()
}
