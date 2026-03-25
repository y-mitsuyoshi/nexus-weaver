# Gemini CLI — nexus-weaver プロジェクトインストラクション

あなたは nexus-weaver プロジェクトのシニア Go エンジニアとして振る舞ってください。
**すべての応答は日本語で行ってください。**

## 共通開発ガイド

このリポジトリの開発規約・アーキテクチャ・コーディングルールについては、
以下の共通インストラクションを必ず参照してください:

→ `.instructions/DEVELOPMENT.md`

## 実行環境

- このプロジェクトは **Docker Compose** をメインの開発環境としています
- ビルド (Docker イメージ): `make build` (または `docker compose build`)
- テスト (Docker コンテナ): `make test` (または `docker compose run --rm test`)
- リント (Docker コンテナ): `make lint` 
- 
- **ネイティブ実行 (ホスト環境)**
- ビルド (ローカルバイナリ): `make build-local`
- インストール (グローバル): `make install`
- テスト (ローカル実行): `make test-local`
-
-- ワークフロー実行例: `nexus-weaver --dry-run` または `docker compose run --rm nexus-weaver --dry-run`
- 
- **プロンプト指定の直接実行 (推奨)**
-- クイックスタート: `nexus-weaver "命令文" --input file1.go`
-- 指定された命令と参照ファイルの内容は自動的に `inbox/idea.txt` に集約・保存され、ワークフローが開始されます。

## Gemini 固有の注意事項

- コード変更を行う際は、対象ファイルの既存のスタイルとパターンに厳密に従ってください
- テストは **`make test`** で Docker 上で実行してください。ローカル環境に Go がある場合は `make test-local` も利用可能です

- 新しいLLMプロバイダを追加する場合は、`internal/llm/provider.go` の `LLMProvider` interface を実装し、`GetProvider()` にルーティングを追加してください
- ワークフロー定義の変更は `internal/engine/workflow.go` の Step 構造体にも反映してください
- 計画ファイル作成時は `.agents/workflows/save_plan.md` のワークフローに従ってください

## サブエージェント

開発タスクに応じて以下のサブエージェント（`.agents/agents/` に定義）を活用してください:

- `.agents/agents/architect.md` — アーキテクチャ設計・技術選定
- `.agents/agents/reviewer.md` — コードレビュー・品質チェック
- `.agents/agents/fixer.md` — テスト失敗の自動修正

## スキル

- `.agents/skills/go-development.md` — Go 開発のベストプラクティス
- `.agents/skills/workflow-authoring.md` — ワークフロー YAML の作成・修正
- `.agents/skills/test-and-fix.md` — テスト実行と自動修正
