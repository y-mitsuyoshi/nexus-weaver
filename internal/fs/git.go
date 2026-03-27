package fs

import (
	"fmt"
	"os/exec"
	"strings"
)

// GetCurrentBranch は現在の Git ブランチ名を返します。
func GetCurrentBranch() (string, error) {
	cmd := exec.Command("git", "rev-parse", "--abbrev-ref", "HEAD")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("failed to get current branch: %w\noutput: %s", err, string(output))
	}
	return strings.TrimSpace(string(output)), nil
}

// IsProtectedBranch は指定されたブランチ名が保護対象（main, master, develop）かどうかを判定します。
func IsProtectedBranch(branch string) bool {
	protected := []string{"main", "master", "develop"}
	for _, p := range protected {
		if branch == p {
			return true
		}
	}
	return false
}

// CreateBranch は新しいブランチを作成し、チェックアウトします。
func CreateBranch(name string) error {
	cmd := exec.Command("git", "checkout", "-b", name)
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to create branch %s: %w\noutput: %s", name, err, string(output))
	}
	return nil
}

// DeleteBranch は指定されたローカルブランチを削除します。
// 現在チェックアウトしているブランチは削除できないため、
// 必要に応じて事前に別ブランチへ切り替えてください。
func DeleteBranch(name string) error {
	cmd := exec.Command("git", "branch", "-D", name)
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to delete branch %s: %w\noutput: %s", name, err, string(output))
	}
	return nil
}

// SwitchBranch は指定されたブランチにチェックアウトします。
func SwitchBranch(name string) error {
	cmd := exec.Command("git", "checkout", name)
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to switch to branch %s: %w\noutput: %s", name, err, string(output))
	}
	return nil
}

// Push は指定されたリモートに現在のブランチをプッシュします。
func Push(remote string) error {
	if remote == "" {
		remote = "origin"
	}
	branch, err := GetCurrentBranch()
	if err != nil {
		return err
	}
	cmd := exec.Command("git", "push", remote, branch)
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to push to %s/%s: %w\noutput: %s", remote, branch, err, string(output))
	}
	return nil
}

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
