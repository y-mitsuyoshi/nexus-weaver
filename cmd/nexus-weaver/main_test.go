package main

import (
	"log/slog"
	"os"
	"path/filepath"
	"strings"
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
	defer func() { _ = os.Chdir(origDir) }()
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("ディレクトリ移動に失敗: %v", err)
	}

	// .nexus/workflow.yml を作成
	nexusDir := filepath.Join(tmpDir, ".nexus")
	if err := os.MkdirAll(nexusDir, 0755); err != nil {
		t.Fatalf("ディレクトリ作成に失敗: %v", err)
	}
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
	defer func() { _ = os.Chdir(origDir) }()
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("ディレクトリ移動に失敗: %v", err)
	}

	// workflows/workflow.yml を作成（.nexus/ は作らない）
	wfDir := filepath.Join(tmpDir, "workflows")
	if err := os.MkdirAll(wfDir, 0755); err != nil {
		t.Fatalf("ディレクトリ作成に失敗: %v", err)
	}
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
	defer func() { _ = os.Chdir(origDir) }()
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("ディレクトリ移動に失敗: %v", err)
	}

	// 両方作成
	if err := os.MkdirAll(filepath.Join(tmpDir, ".nexus"), 0755); err != nil {
		t.Fatalf("ディレクトリ作成に失敗: %v", err)
	}
	if err := os.WriteFile(filepath.Join(tmpDir, ".nexus", "workflow.yml"), []byte("name: nexus"), 0644); err != nil {
		t.Fatalf("ファイル作成に失敗: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(tmpDir, "workflows"), 0755); err != nil {
		t.Fatalf("ディレクトリ作成に失敗: %v", err)
	}
	if err := os.WriteFile(filepath.Join(tmpDir, "workflows", "workflow.yml"), []byte("name: workflows"), 0644); err != nil {
		t.Fatalf("ファイル作成に失敗: %v", err)
	}

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
	defer func() { _ = os.Chdir(origDir) }()
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("ディレクトリ移動に失敗: %v", err)
	}

	_, err := resolveWorkflowPath("")
	if err == nil {
		t.Error("ファイルが見つからない場合はエラーが返されるべきです")
	}
}

// TestRunInit は .nexus/ ディレクトリの初期化テストです。
func TestRunInit(t *testing.T) {
	tmpDir := t.TempDir()
	origDir, _ := os.Getwd()
	defer func() { _ = os.Chdir(origDir) }()
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("ディレクトリ移動に失敗: %v", err)
	}

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
	defer func() { _ = os.Chdir(origDir) }()
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("ディレクトリ移動に失敗: %v", err)
	}

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

// TestSaveInitialPrompt_PromptOnly はプロンプトのみの場合のテストです。
func TestSaveInitialPrompt_PromptOnly(t *testing.T) {
	tmpDir := t.TempDir()
	origDir, _ := os.Getwd()
	defer func() { _ = os.Chdir(origDir) }()
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("ディレクトリ移動に失敗: %v", err)
	}

	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelWarn,
	}))

	if err := saveInitialPrompt(logger, "ユーザー一覧APIを作って", nil); err != nil {
		t.Fatalf("saveInitialPrompt() がエラーを返しました: %v", err)
	}

	content, err := os.ReadFile("inbox/idea.txt")
	if err != nil {
		t.Fatalf("inbox/idea.txt の読み込みに失敗: %v", err)
	}

	text := string(content)
	if !strings.Contains(text, "# タスク") {
		t.Error("ヘッダー '# タスク' が含まれていません")
	}
	if !strings.Contains(text, "ユーザー一覧APIを作って") {
		t.Error("プロンプト内容が含まれていません")
	}
	if strings.Contains(text, "# 参照ファイル") {
		t.Error("参照ファイルが無い場合にセクションが生成されるべきではない")
	}
}

// TestSaveInitialPrompt_WithRefs はプロンプト+参照ファイルの場合のテストです。
func TestSaveInitialPrompt_WithRefs(t *testing.T) {
	tmpDir := t.TempDir()
	origDir, _ := os.Getwd()
	defer func() { _ = os.Chdir(origDir) }()
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("ディレクトリ移動に失敗: %v", err)
	}

	// 参照ファイルを作成
	refPath := filepath.Join(tmpDir, "spec.md")
	if err := os.WriteFile(refPath, []byte("# API仕様\n\nGET /users"), 0644); err != nil {
		t.Fatalf("参照ファイルの作成に失敗: %v", err)
	}

	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelWarn,
	}))

	if err := saveInitialPrompt(logger, "この仕様に基づいて実装して", []string{refPath}); err != nil {
		t.Fatalf("saveInitialPrompt() がエラーを返しました: %v", err)
	}

	content, err := os.ReadFile("inbox/idea.txt")
	if err != nil {
		t.Fatalf("inbox/idea.txt の読み込みに失敗: %v", err)
	}

	text := string(content)
	if !strings.Contains(text, "# 参照ファイル") {
		t.Error("参照ファイルセクションが含まれていません")
	}
	if !strings.Contains(text, "GET /users") {
		t.Error("参照ファイルの内容が含まれていません")
	}
}

// TestSaveInitialPrompt_RefNotFound は存在しない参照ファイルでエラーを返すテストです。
func TestSaveInitialPrompt_RefNotFound(t *testing.T) {
	tmpDir := t.TempDir()
	origDir, _ := os.Getwd()
	defer os.Chdir(origDir)
	os.Chdir(tmpDir)

	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelWarn,
	}))

	err := saveInitialPrompt(logger, "テスト", []string{"/nonexistent/file.txt"})
	if err == nil {
		t.Error("存在しない参照ファイルでエラーが返されるべきです")
	}
}
