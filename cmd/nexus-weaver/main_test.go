package main

import (
	"log/slog"
	"os"
	"path/filepath"
	"testing"
)

// TestResolveWorkflowPath_Explicit は明示的にパスを指定した場合のテストです。
func TestResolveWorkflowPath_Explicit(t *testing.T) {
	// 一時ディレクトリにテスト用ワークフローファイルを作成
	tmpDir := t.TempDir()
	wfPath := filepath.Join(tmpDir, "custom.yml")
	if err := os.WriteFile(wfPath, []byte("name: test"), 0644); err != nil {
		t.Fatalf("テストファイルの作成に失敗: %v", err)
	}

	result, err := resolveWorkflowPath(wfPath)
	if err != nil {
		t.Fatalf("resolveWorkflowPath() がエラーを返しました: %v", err)
	}
	if result != wfPath {
		t.Errorf("期待値 %q, 結果 %q", wfPath, result)
	}
}

// TestResolveWorkflowPath_ExplicitNotFound は存在しないファイルを指定した場合のテストです。
func TestResolveWorkflowPath_ExplicitNotFound(t *testing.T) {
	_, err := resolveWorkflowPath("/nonexistent/workflow.yml")
	if err == nil {
		t.Error("存在しないファイルでエラーが返されるべきです")
	}
}

// TestResolveWorkflowPath_DefaultNexus は .nexus/workflow.yml が自動検出されるテストです。
func TestResolveWorkflowPath_DefaultNexus(t *testing.T) {
	// テスト用の一時ディレクトリで実行
	tmpDir := t.TempDir()
	origDir, _ := os.Getwd()
	defer os.Chdir(origDir)
	os.Chdir(tmpDir)

	// .nexus/workflow.yml を作成
	nexusDir := filepath.Join(tmpDir, ".nexus")
	os.MkdirAll(nexusDir, 0755)
	if err := os.WriteFile(filepath.Join(nexusDir, "workflow.yml"), []byte("name: test"), 0644); err != nil {
		t.Fatalf("テストファイルの作成に失敗: %v", err)
	}

	result, err := resolveWorkflowPath("")
	if err != nil {
		t.Fatalf("resolveWorkflowPath() がエラーを返しました: %v", err)
	}
	if result != ".nexus/workflow.yml" {
		t.Errorf("期待値 .nexus/workflow.yml, 結果 %q", result)
	}
}

// TestResolveWorkflowPath_DefaultWorkflows は workflows/workflow.yml がフォールバックされるテストです。
func TestResolveWorkflowPath_DefaultWorkflows(t *testing.T) {
	tmpDir := t.TempDir()
	origDir, _ := os.Getwd()
	defer os.Chdir(origDir)
	os.Chdir(tmpDir)

	// workflows/workflow.yml を作成（.nexus/ は作らない）
	wfDir := filepath.Join(tmpDir, "workflows")
	os.MkdirAll(wfDir, 0755)
	if err := os.WriteFile(filepath.Join(wfDir, "workflow.yml"), []byte("name: test"), 0644); err != nil {
		t.Fatalf("テストファイルの作成に失敗: %v", err)
	}

	result, err := resolveWorkflowPath("")
	if err != nil {
		t.Fatalf("resolveWorkflowPath() がエラーを返しました: %v", err)
	}
	if result != "workflows/workflow.yml" {
		t.Errorf("期待値 workflows/workflow.yml, 結果 %q", result)
	}
}

// TestResolveWorkflowPath_NexusPriority は .nexus/ が workflows/ より優先されるテストです。
func TestResolveWorkflowPath_NexusPriority(t *testing.T) {
	tmpDir := t.TempDir()
	origDir, _ := os.Getwd()
	defer os.Chdir(origDir)
	os.Chdir(tmpDir)

	// 両方作成
	os.MkdirAll(filepath.Join(tmpDir, ".nexus"), 0755)
	os.WriteFile(filepath.Join(tmpDir, ".nexus", "workflow.yml"), []byte("name: nexus"), 0644)
	os.MkdirAll(filepath.Join(tmpDir, "workflows"), 0755)
	os.WriteFile(filepath.Join(tmpDir, "workflows", "workflow.yml"), []byte("name: workflows"), 0644)

	result, err := resolveWorkflowPath("")
	if err != nil {
		t.Fatalf("resolveWorkflowPath() がエラーを返しました: %v", err)
	}
	if result != ".nexus/workflow.yml" {
		t.Errorf(".nexus/ が優先されるべき。期待値 .nexus/workflow.yml, 結果 %q", result)
	}
}

// TestResolveWorkflowPath_NoFile はどのファイルも存在しない場合のテストです。
func TestResolveWorkflowPath_NoFile(t *testing.T) {
	tmpDir := t.TempDir()
	origDir, _ := os.Getwd()
	defer os.Chdir(origDir)
	os.Chdir(tmpDir)

	_, err := resolveWorkflowPath("")
	if err == nil {
		t.Error("ファイルが見つからない場合はエラーが返されるべきです")
	}
}

// TestRunInit は .nexus/ ディレクトリの初期化テストです。
func TestRunInit(t *testing.T) {
	tmpDir := t.TempDir()
	origDir, _ := os.Getwd()
	defer os.Chdir(origDir)
	os.Chdir(tmpDir)

	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelWarn, // テスト中はログを抑制
	}))

	if err := runInit(logger); err != nil {
		t.Fatalf("runInit() がエラーを返しました: %v", err)
	}

	// .nexus/workflow.yml が存在するか
	if _, err := os.Stat(filepath.Join(tmpDir, ".nexus", "workflow.yml")); os.IsNotExist(err) {
		t.Error(".nexus/workflow.yml が作成されていません")
	}

	// .nexus/prompts/ が存在するか
	if _, err := os.Stat(filepath.Join(tmpDir, ".nexus", "prompts")); os.IsNotExist(err) {
		t.Error(".nexus/prompts/ ディレクトリが作成されていません")
	}
}

// TestRunInit_Idempotent は二重実行でエラーにならないことを確認するテストです。
func TestRunInit_Idempotent(t *testing.T) {
	tmpDir := t.TempDir()
	origDir, _ := os.Getwd()
	defer os.Chdir(origDir)
	os.Chdir(tmpDir)

	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelWarn,
	}))

	// 1回目
	if err := runInit(logger); err != nil {
		t.Fatalf("1回目の runInit() がエラーを返しました: %v", err)
	}

	// 2回目（冪等性の確認）
	if err := runInit(logger); err != nil {
		t.Fatalf("2回目の runInit() がエラーを返しました: %v", err)
	}
}

// TestDefaultWorkflowPaths はデフォルト探索パスの定義を検証します。
func TestDefaultWorkflowPaths(t *testing.T) {
	expected := []string{
		".nexus/workflow.yml",
		".nexus/workflow.yaml",
		"workflows/workflow.yml",
		"workflows/workflow.yaml",
	}

	if len(defaultWorkflowPaths) != len(expected) {
		t.Fatalf("defaultWorkflowPaths の数が異なる。期待 %d, 結果 %d", len(expected), len(defaultWorkflowPaths))
	}

	for i, path := range defaultWorkflowPaths {
		if path != expected[i] {
			t.Errorf("defaultWorkflowPaths[%d]: 期待 %q, 結果 %q", i, expected[i], path)
		}
	}
}
