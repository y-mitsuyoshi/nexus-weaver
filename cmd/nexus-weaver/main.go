package main

import (
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
)

// デフォルトで探索するワークフローパスの順序
var defaultWorkflowPaths = []string{
	".nexus/workflow.yml",
	".nexus/workflow.yaml",
	"workflows/workflow.yml",
	"workflows/workflow.yaml",
}

// resolveWorkflowPath は与えられたパスが空文字列ならデフォルト候補を順に探索し、
// 見つかった相対パスを返します。明示的にパスが与えられた場合はその存在を検証します。
func resolveWorkflowPath(path string) (string, error) {
	if path != "" {
		if _, err := os.Stat(path); err == nil {
			return path, nil
		} else {
			return "", fmt.Errorf("workflow file not found: %w", err)
		}
	}

	for _, p := range defaultWorkflowPaths {
		if _, err := os.Stat(p); err == nil {
			return p, nil
		}
	}

	return "", errors.New("no workflow file found")
}

// runInit は .nexus ディレクトリ構成を初期化します。冪等に動作します。
func runInit(logger *slog.Logger) error {
	nexusDir := ".nexus"
	promptsDir := filepath.Join(nexusDir, "prompts")
	if err := os.MkdirAll(promptsDir, 0755); err != nil {
		if logger != nil {
			logger.Warn("failed to create .nexus/prompts", "err", err)
		}
		return err
	}

	wfPath := filepath.Join(nexusDir, "workflow.yml")
	if _, err := os.Stat(wfPath); os.IsNotExist(err) {
		if err := os.WriteFile(wfPath, []byte("name: nexus-weaver\n"), 0644); err != nil {
			if logger != nil {
				logger.Warn("failed to write workflow.yml", "err", err)
			}
			return err
		}
	}

	return nil
}

// saveInitialPrompt は inbox/idea.txt にタスクと参照ファイルを保存します。
// refs に存在しないファイルが含まれる場合はエラーを返します。
func saveInitialPrompt(logger *slog.Logger, prompt string, refs []string) error {
	if err := os.MkdirAll("inbox", 0755); err != nil {
		if logger != nil {
			logger.Warn("failed to create inbox dir", "err", err)
		}
		return err
	}

	var b strings.Builder
	b.WriteString("# タスク\n\n")
	b.WriteString(prompt)
	b.WriteString("\n\n")

	if len(refs) > 0 {
		b.WriteString("# 参照ファイル\n\n")
		for _, r := range refs {
			content, err := os.ReadFile(r)
			if err != nil {
				return fmt.Errorf("failed to read ref %s: %w", r, err)
			}
			b.WriteString(string(content))
			b.WriteString("\n\n")
		}
	}

	if err := os.WriteFile(filepath.Join("inbox", "idea.txt"), []byte(b.String()), 0644); err != nil {
		if logger != nil {
			logger.Warn("failed to write idea.txt", "err", err)
		}
		return err
	}

	return nil
}