# GitHub Copilot CLI — nexus-weaver プロジェクトインストラクション

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

## Git 運用ルール

- **main ブランチへの直接コミット禁止**。新規変更は必ず feature ブランチ（`feature/`, `fix/`, `improvement/`）を作成してから行ってください
- テスト実行前に変更をコミットしてください（テスト中のブランチ操作による変更消失を防止）
- テスト完了後にブランチを main に戻す必要はありません

## Copilot 固有の注意事項

- コード変更を行う際は、対象ファイルの既存のスタイルとパターンに厳密に従ってください
- テストは **`make test`** で Docker 上で実行してください。ローカル環境に Go がある場合は `make test-local` も利用可能です

- 新しいLLMプロバイダを追加する場合は、`internal/llm/provider.go` の `LLMProvider` interface を実装し、`GetProvider()` にルーティングを追加してください
- ワークフロー定義の変更は `internal/engine/workflow.go` の Step 構造体にも反映してください
- 計画ファイル作成時は `.agents/workflows/save_plan.md` のワークフローに従ってください

## サブエージェント

このリポジトリには以下のカスタムエージェントが定義されています
（実体は `.agents/agents/` に共有ファイルとして配置し、`.github/agents/` からシンボリックリンク）:

- `@architect` — アーキテクチャ設計・技術選定
- `@reviewer` — コードレビュー・品質チェック
- `@fixer` — テスト失敗の自動修正

## スキル

開発タスクに応じて以下のスキルを活用してください:

- `.agents/skills/go-development.md` — Go 開発のベストプラクティス
- `.agents/skills/workflow-authoring.md` — ワークフロー YAML の作成・修正
- `.agents/skills/test-and-fix.md` — テスト実行と自動修正
