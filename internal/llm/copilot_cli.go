package llm

import (
	"bytes"
	"fmt"
	"os/exec"
	"strings"
)

// CopilotCLI は GitHub Copilot CLI (copilot コマンド) を利用するプロバイダです。
// `copilot` コマンドを os/exec で実行し、標準出力を応答として返します。
type CopilotCLI struct {
	Model string
}

// Generate はシステムプロンプトとユーザープロンプトを結合して
// GitHub Copilot CLI にワンショット実行（-p オプション）で渡し、応答テキストを返します。
func (c *CopilotCLI) Generate(systemPrompt, userPrompt string) (string, error) {
	// copilot コマンドの存在確認
	if _, err := exec.LookPath("copilot"); err != nil {
		return "", fmt.Errorf("'copilot' command not found in PATH. Please install it first")
	}

	// プロンプトを構造化して結合
	combinedPrompt := formatPrompt(systemPrompt, userPrompt)

	// ワンショット実行: -p オプションでプロンプトを渡す
	args := []string{"-p", combinedPrompt}
	if c.Model != "" {
		args = append(args, "--model", c.Model)
	}
	cmd := exec.Command("copilot", args...)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("GitHub Copilot CLI execution failed: %w\nstderr: %s", err, stderr.String())
	}

	return strings.TrimSpace(stdout.String()), nil
}
