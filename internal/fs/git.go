package fs

import (
	"fmt"
	"os/exec"
	"strings"
)

// AutoCommit はステージされていない全変更を Git に自動コミットします。
// 戻り値はコミット作成前の HEAD のコミットハッシュ（存在しなければ空文字列）です。
// テスト実行前の安全装置として、ループ暴走時のロールバックに備えます。
func AutoCommit(message string) (string, error) {
	// git が使えるか、ワークツリー内かを確認
	if output, err := exec.Command("git", "rev-parse", "--is-inside-work-tree").CombinedOutput(); err != nil {
		return "", fmt.Errorf("git rev-parse failed: %w\noutput: %s", err, string(output))
	} else if strings.TrimSpace(string(output)) != "true" {
		return "", fmt.Errorf("not inside a git worktree")
	}

	// 現在の HEAD を取得（初期コミットがない場合は失敗する）
	headOut, headErr := exec.Command("git", "rev-parse", "HEAD").CombinedOutput()
	baseHash := ""
	if headErr == nil {
		baseHash = strings.TrimSpace(string(headOut))
	}

	// 変更があるかチェック
	statusOut, statusErr := exec.Command("git", "status", "--porcelain").CombinedOutput()
	if statusErr != nil {
		return baseHash, fmt.Errorf("git status failed: %w\noutput: %s", statusErr, string(statusOut))
	}
	if strings.TrimSpace(string(statusOut)) == "" {
		// 変更なし: コミットせずにそのまま返す
		return baseHash, nil
	}

	// 全変更をステージング
	if output, err := exec.Command("git", "add", ".").CombinedOutput(); err != nil {
		return baseHash, fmt.Errorf("git add failed: %w\noutput: %s", err, string(output))
	}

	// コミット
	if output, err := exec.Command("git", "commit", "-m", message).CombinedOutput(); err != nil {
		return baseHash, fmt.Errorf("git commit failed: %w\noutput: %s", err, string(output))
	}

	return baseHash, nil
}

// Rollback は指定したコミットハッシュにワークツリーをリセットします。
// テストループが暴走した場合の緊急安全装置です。
func Rollback(target string) error {
	if target == "" {
		return fmt.Errorf("no target commit specified for rollback")
	}
	cmd := exec.Command("git", "reset", "--hard", target)
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("git rollback failed: %w\noutput: %s", err, string(output))
	}
	return nil
}
