package llm

import (
	"bytes"
	"fmt"
	"os/exec"
	"strings"
)

// CopilotCLI は GitHub Copilot CLI (copilot コマンド) を利用するプロバイダです。
// `copilot` コマンドを os/exec で実行し、標準出力を応答として返します。
type CopilotCLI struct{}

// Generate はシステムプロンプトとユーザープロンプトを結合して
// GitHub Copilot CLI に渡し、応答テキストを返します。
func (c *CopilotCLI) Generate(systemPrompt, userPrompt string) (string, error) {
	// GitHub Copilot CLI は chat モードなどで対話的に動作することが多いですが、
	// ここでは非対話的な生成を想定したプロンプト構築を行います。
	// Copilot CLI の仕様に合わせて、プロンプトを単一の入力としてまとめます。
	combinedPrompt := formatPrompt(systemPrompt, userPrompt)

	// copilot chat または直接プロンプトを渡せるサブコマンドを想定
	// 現時点の GA 版に基づき、標準的な chat 実行コマンドを構成します
	// (実際のエイリアスやパスは環境によりますが、指示に従い 'copilot' コマンドを使用)
	cmd := exec.Command("copilot", "chat", combinedPrompt, "--no-interactive")

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		// フォールバック: 'chat' サブコマンドがない古いバージョンの可能性などを考慮
		// または単純に 'copilot' に直接渡す形を試行
		cmdFallback := exec.Command("copilot", combinedPrompt)
		stdout.Reset()
		stderr.Reset()
		cmdFallback.Stdout = &stdout
		cmdFallback.Stderr = &stderr

		if errFallback := cmdFallback.Run(); errFallback != nil {
			return "", fmt.Errorf("GitHub Copilot CLI execution failed: %w\nstderr: %s", errFallback, stderr.String())
		}
	}

	return strings.TrimSpace(stdout.String()), nil
}
