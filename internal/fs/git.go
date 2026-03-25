package fs

import (
	"fmt"
	"os/exec"
)

// AutoCommit はステージされていない全変更を Git に自動コミットします。
// テスト実行前の安全装置として、ループ暴走時のロールバックに備えます。
func AutoCommit(message string) error {
	// 全変更をステージング
	addCmd := exec.Command("git", "add", ".")
	if output, err := addCmd.CombinedOutput(); err != nil {
		return fmt.Errorf("git add failed: %w\noutput: %s", err, string(output))
	}

	// コミット（変更がない場合もエラーにしない）
	commitCmd := exec.Command("git", "commit", "-m", message, "--allow-empty")
	if output, err := commitCmd.CombinedOutput(); err != nil {
		return fmt.Errorf("git commit failed: %w\noutput: %s", err, string(output))
	}

	return nil
}

// Rollback は直前のコミットを取り消し、作業ディレクトリを1つ前の状態に戻します。
// テストループが暴走した場合の緊急安全装置です。
func Rollback() error {
	cmd := exec.Command("git", "reset", "--hard", "HEAD~1")
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("git rollback failed: %w\noutput: %s", err, string(output))
	}
	return nil
}
