# nexus-weaver

`nexus-weaver` は、YAML で定義したワークフローに従って複数の LLM（エージェント）を切り替えながら処理を進める、Go 製の CLI オーケストレーターです。

断片的なアイデア（テキスト）から始まり、PRDの作成、アーキテクチャ設計、テスト駆動の自動実装、修正ループ（Lint/Test）、AIコードレビュー、そして Pull Request の作成までを一貫したパイプラインとして全自動または半自動で実行できます。

## 開発ワークフロー（Feature Development Pipeline）

本リポジトリに組み込まれている標準のワークフロー（`workflows/workflow.yml`）は、以下の強力なループで構成されています：

1. **ブランチ作成**: `inbox/idea.txt` からブランチ名を自動生成しチェックアウト
2. **要件・設計定義**: プロダクトマネージャー（PM）が PRD を作成し、アーキテクトが設計書を作成
3. **ドキュメントレビュー**: 生成された PRD とアーキテクチャ設計書をそれぞれレビューし、必要に応じて自動修正
4. **テスト駆動実装**: テストエンジニアが設計書からテストコード（`main_test.go`）を生成し、エンジニアが実装コード（`main.go`）を生成
5. **フォーマット＆静的解析**: `go fmt` および `go vet` による整形
6. **テスト・修正ループ (`test_and_fix_loop`)**: テストを実行し、失敗した場合は Fixer モデルが自動でエラーを解析して修正。成功するまで指定回数リトライ
7. **レビューゲート**: コード・QA・セキュリティの多角的な視点で AI がレビュー。指摘があれば Fixer が自動修正を行い、全てのレビューを再度通過するまでループ（ユーザーによる対話的な Approve/Reject も可能）
8. **PR作成**: テックリード（TechLead）ロールが PRD を元に適切な PR タイトルと説明文を生成し、GitHub CLI (`gh`) を通じて自動で Pull Request を作成

## ディレクトリ構成

```text
nexus-weaver/
├── cmd/nexus-weaver/     # CLI エントリーポイント
├── internal/
│   ├── engine/           # ワークフローパーサー & 実行エンジン
│   ├── fs/               # ファイル I/O と Git ユーティリティ
│   └── llm/              # LLM プロバイダ実装
├── prompts/              # システムプロンプト (.md) 各エージェントの役割
│   ├── architect.md             # アーキテクチャ設計
│   ├── architecture_reviewer.md # アーキテクチャ設計レビュー (New!)
│   ├── branch_namer.md          # ブランチ命名
│   ├── document_fixer.md        # ドキュメント修正 (New!)
│   ├── engineer.md              # 実装
│   ├── fixer.md                 # エラー・コード修正
│   ├── pm.md                    # プロダクト要件定義
│   ├── prd_reviewer.md          # PRDレビュー (New!)
│   ├── pr_body.md               # PR説明文の生成
│   ├── pr_title.md              # PRタイトルの生成
│   ├── qa.md                    # QAレビュー
│   ├── reviewer.md              # コードレビュー
│   ├── security.md              # セキュリティレビュー
│   └── test_engineer.md         # テストコード生成
├── workflows/            # ワークフロー定義 YAML
├── inbox/                # 初期入力ファイル置き場 (idea.txt など)
└── docs/                 # 生成物 (PRD, Architecture, PR内容など) が出力されるディレクトリ
```

## 前提条件

### 共通

- Go 1.24 以上
- Git
- GitHub CLI (`gh`) ※ 最後のPR作成ステップを実行するために必要

### 利用する LLM に応じて必要なもの

- `gemini-cli` を使う場合: `gemini` コマンドが実行できること
- `copilot-cli` を使う場合: `copilot` コマンドが実行できること
- `local-qwen` を使う場合: OpenAI 互換 API が稼働していること。エンドポイントは環境変数 `LOCAL_QWEN_ENDPOINT` で指定可能（デフォルト: `http://localhost:11434/v1/chat/completions`）。

## セットアップ

### ローカルでビルド・実行する

```bash
go build -o nexus-weaver ./cmd/nexus-weaver
```

## 基本的な使い方

### 0. アイデアを配置する

`inbox/idea.txt` に、実装したい機能のアイデアや要望を記述します。

### 1. ワークフローを確認する

まずはドライランで YAML の読み込みとバリデーションだけを確認します。

```bash
./nexus-weaver --workflow workflows/workflow.yml --dry-run
```

### 2. ワークフローを実行する

```bash
./nexus-weaver --workflow workflows/workflow.yml
```

途中のレビューゲートでは、AI のレビュー結果が画面に表示され、ユーザーに承認（Approve）、修正依頼（Fix）、または拒否（Reject）を求めます。

## ステップ種別

### `llm_task`
LLM にテキスト生成を依頼し、その結果を `output_file` に保存します。出力が `.go` や `.yml` の場合、Markdown のコードブロックを自動抽出します。

### `loop`
テストや検証コマンドを実行し、失敗した場合に Fixer モデルを呼び出します。
各リトライ前に `AutoCommit` が行われ、最終失敗時には `Rollback` が実行されます。

### `git_branch`
新しい Git ブランチを作成し、チェックアウトします。保護ブランチでの直接実行を防ぐため、これらのブランチから開始する場合はこのステップが必須です。
`branch_name` または `branch_name_file` でブランチ名を指定します。

### `review`
LLM によるコードレビューを実行し、ユーザーに対話的な承認を求めます。ユーザーが修正 (`[f]ix`) を指示すると自動でコードを修正し、レビューゲートの最初から再検証します。

### `git_push`
現在のブランチを指定されたリモートにプッシュします。

### `command_task`
任意のシェルコマンドを実行します。フォーマットや PR 作成などに活用します。

## License

See [LICENSE](LICENSE) file.
