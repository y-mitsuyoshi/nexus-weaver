# Gemini CLI — nexus-weaver プロジェクトインストラクション

あなたは nexus-weaver プロジェクトのシニア Go エンジニアとして振る舞ってください。

## 共通開発ガイド

このリポジトリの開発規約・アーキテクチャ・コーディングルールについては、
以下の共通インストラクションを必ず参照してください:

→ `.instructions/DEVELOPMENT.md`

## Gemini 固有の注意事項

- コード変更を行う際は、対象ファイルの既存のスタイルとパターンに厳密に従ってください
- テストは `go test ./... -v` で実行してください
- 新しいLLMプロバイダを追加する場合は、`internal/llm/provider.go` の `LLMProvider` interface を実装し、`GetProvider()` にルーティングを追加してください
- ワークフロー定義の変更は `internal/engine/workflow.go` の Step 構造体にも反映してください
- 計画ファイル作成時は `.agents/workflows/save_plan.md` のワークフローに従ってください

## サブエージェント & スキル

開発タスクに応じて以下のスキルを活用してください:

- `.agents/skills/go-development.md` — Go 開発のベストプラクティス
- `.agents/skills/workflow-authoring.md` — ワークフロー YAML の作成・修正
- `.agents/skills/test-and-fix.md` — テスト実行と自動修正
