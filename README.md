# nexus-weaver

`nexus-weaver` は、YAML で定義したワークフローに従って複数の LLM を切り替えながら処理を進める、Go 製の CLI オーケストレーターです。

PRD 作成、設計、実装、テスト実行、コードレビュー、ブランチ作成、リモートへのPush、コマンド実行といった工程を 1 本のパイプラインとして表現できます。

## できること

- YAML でワークフローを定義する
- ステップごとに `gemini-cli` / `copilot-cli` / `local-qwen` を切り替える
- 入力ファイルを読み、LLM の出力をファイルへ保存する
- 新規 Git ブランチを作成する (`git_branch`)
- レビュアーLLMによるコードレビューと対話的な自動修正 (`review`)
- テストコマンドをリトライ付きで実行し、失敗時に自動修正する (`loop`)
- 変更をリモートリポジトリにプッシュする (`git_push`)
- 任意のシェルコマンドをワークフローの任意の箇所に組み込む (`command_task`)

## 現在の実装状況

LLM を用いたタスク実行 (`llm_task`)、テストの自動修正ループ (`loop`) に加えて、以下の機能が実装されています。
- `git_branch`: 動的に生成したブランチ名での新規ブランチ作成
- `review`: LLMによるファイル内容のレビューと対話的な承認・修正プロセス。連続する review ステップは「レビューゲート」としてグループ化され、いずれかのレビューで修正が発生した場合は、全体の一貫性を保つためにゲート全体のレビューを最初からやり直します。
- `git_push`: リモートへのプッシュ

`loop` や `review` ステップでは、実行前の状態をコミット（AutoCommit）し、複数回のリトライが失敗したりユーザーがリジェクトした場合には、自動的にロールバックが行われます。

## ディレクトリ構成

```text
nexus-weaver/
├── cmd/nexus-weaver/     # CLI エントリーポイント
├── internal/
│   ├── engine/           # ワークフローパーサー & 実行エンジン
│   ├── fs/               # ファイル I/O と Git ユーティリティ
│   └── llm/              # LLM プロバイダ実装
├── prompts/              # システムプロンプト
├── workflows/            # ワークフロー定義 YAML
└── docs/                 # 生成物や設計書
```

## 前提条件

### 共通

- Go 1.24 以上
- Git

### 利用する LLM に応じて必要なもの

- `gemini-cli` を使う場合: `gemini` コマンドが実行できること
- `copilot-cli` を使う場合: `copilot` コマンドが実行できること
- `local-qwen` を使う場合: OpenAI 互換 API が稼働していること。エンドポイントは環境変数 `LOCAL_QWEN_ENDPOINT` で指定可能（デフォルト: `http://localhost:11434/v1/chat/completions`）。

`internal/llm/provider.go` はこの環境変数を参照してエンドポイントを決定します。

## セットアップ

### ローカルで実行する

```bash
go build -o nexus-weaver ./cmd/nexus-weaver
```

### Docker Compose で実行する

```bash
docker compose build
```

`docker-compose.yml` はカレントディレクトリを `/app` にマウントし、`network_mode: "host"` を使用しています（Linux 環境でのみ有効です）。これはコンテナからホストの `localhost:11434`（local-qwen）へ到達するためです。Mac/Windows の Docker Desktop では host ネットワークが制限されるため、別のネットワーク方法が必要です。

## 基本的な使い方

### 1. ワークフローを確認する

まずはドライランで YAML の読み込みとバリデーションだけを確認します。

```bash
./nexus-weaver --workflow workflows/workflow.yml --dry-run
```

Docker Compose の場合:

```bash
docker compose run --rm nexus-weaver --dry-run
```

### 2. ワークフローを実行する

```bash
./nexus-weaver --workflow workflows/workflow.yml
```

Docker Compose の場合:

```bash
docker compose run --rm nexus-weaver
```

### 3. 詳細ログを有効にする

```bash
./nexus-weaver --workflow workflows/workflow.yml --verbose
```

## CLI オプション

| Flag | Default | Description |
|------|---------|-------------|
| `--workflow` | `workflows/workflow.yml` | ワークフロー定義 YAML のパス |
| `--verbose` | `false` | 詳細ログを出力する |
| `--dry-run` | `false` | 読み込みと検証のみ行い、実行はしない |

## ワークフローの書き方

ワークフローは `name` と `steps` で構成されます。

サンプル:

```yaml
name: "Feature Development Pipeline"
steps:
  - id: "generate_branch_name"
    type: "llm_task"
    agent_role: "ReleaseEngineer"
    model: "gemini-cli"
    system_prompt_file: "./prompts/branch_namer.md"
    input_file: "./inbox/idea.txt"
    output_file: "./docs/branch_name.txt"

  - id: "branch_creation"
    type: "git_branch"
    branch_name_file: "./docs/branch_name.txt"

  - id: "prd_generation"
    type: "llm_task"
    agent_role: "ProductManager"
    model: "gemini-cli"
    system_prompt_file: "./prompts/pm.md"
    input_file: "./inbox/idea.txt"
    output_file: "./docs/prd.md"

  - id: "prd_review"
    type: "review"
    reviewer_model: "gemini-cli"
    review_prompt_file: "./prompts/prd_reviewer.md"
    target_file: "./docs/prd.md"
    max_retries: 3
    fixer_model: "gemini-cli"
    fixer_prompt_file: "./prompts/document_fixer.md"

  - id: "push_to_remote"
    type: "git_push"
    remote: "origin"
```

## ステップ種別

### `llm_task`

LLM にテキスト生成を依頼し、その結果を `output_file` に保存します。出力ファイルの拡張子（`.go`, `.yml` など）に応じて自動的にコードブロックが抽出されます。

主なフィールド:

- `id`: ステップの一意な識別子
- `type`: `llm_task`
- `agent_role`: ログ出力用の役割名
- `model`: `gemini-cli` / `copilot-cli` / `local-qwen`
- `system_prompt_file`: システムプロンプトのファイル
- `input_file`: 入力ファイル
- `output_file`: 出力先ファイル

### `loop`

テストや検証コマンドを実行し、失敗した場合に Fixer モデルを呼び出します。

主なフィールド:

- `id`: ステップの一意な識別子
- `type`: `loop`
- `command`: 実行するコマンド
- `max_retries`: 最大試行回数
- `fixer_model`: 失敗時に呼ぶモデル
- `fixer_prompt_file`: Fixer 用システムプロンプト
- `target_file`: 修正対象のファイル

注意点:

- 各リトライ前に Git 自動コミット（AutoCommit）が行われます。これは変更を保護するための安全装置です
- 最終リトライまで失敗した場合、直前のコミットに戻すために Git の Rollback（`git reset --hard <HASH>`）が実行されます。
- Fixer の応答からフェンスドコードブロックが抽出され、`target_file` に自動適用されます。

### `git_branch`

新しい Git ブランチを作成します。保護ブランチ（main, master など）での直接実行を避けるために利用できます。

主なフィールド:

- `id`: ステップの一意な識別子
- `type`: `git_branch`
- `branch_name`: 作成するブランチ名（直接指定）
- `branch_name_file`: ブランチ名が記載されたファイルのパス（`llm_task`の出力などを指定）

注意点:
- 指定されたブランチ名に `feature/`, `fix/`, `improvement/` のプレフィックスがない場合、自動的に `feature/` が付与されます。

### `review`

対象ファイルを LLM にレビューさせ、対話的な承認プロセスを提供します。

主なフィールド:

- `id`: ステップの一意な識別子
- `type`: `review`
- `reviewer_model`: レビューを行うモデル
- `review_prompt_file`: レビュアー用システムプロンプト
- `target_file`: レビュー対象のファイル
- `max_retries`: レビューゲートの最大試行回数（省略可）
- `fixer_model`: 修正時に呼ぶモデル（省略時は `reviewer_model` が使用されます）
- `fixer_prompt_file`: Fixer 用システムプロンプト

注意点:
- 実行時に `[a]pprove / [f]ix & approve / [r]eject / [q]uit` の選択肢が提示されます。
- `f` (fix & approve) を選択すると、`fixer_model` に指摘内容と修正を依頼し、`target_file` に適用します。
- 連続する `review` ステップは「レビューゲート」として扱われ、いずれかで修正が発生するとゲート内の全レビューを最初からやり直します。
- ゲート開始前に AutoCommit が行われ、リジェクト時やリトライ上限到達時にはロールバックされます。
- レビュー結果は `docs/reviews/review-<ID>.md` に保存されます。

### `git_push`

現在のブランチをリモートリポジトリにプッシュします。

主なフィールド:

- `id`: ステップの一意な識別子
- `type`: `git_push`
- `remote`: プッシュ先のリモート名（省略時は `origin`）

### `command_task`

任意のシェルコマンドを実行します。PR 作成や補助スクリプト実行などに使えます。

主なフィールド:

- `id`: ステップの一意な識別子
- `type`: `command_task`
- `command`: `sh -c` で実行するコマンド

## 実行の流れ

1. CLI がワークフロー YAML を読み込む
2. 各ステップの定義を検証し、保護ブランチのチェックを行う（直接実行の防止）
3. `git_branch` で新しいブランチを作成する（定義されている場合）
4. `llm_task` は入力ファイルを読み、LLM の出力をファイルへ保存する
5. `loop` はコマンドを実行し、失敗時は Fixer モデルを呼ぶ
6. `review` は対話的なレビューを実施し、必要に応じて自動修正・再レビュー（レビューゲート）を行う
7. `git_push` でリモートにプッシュする
8. `command_task` は指定コマンドをそのまま実行する

## 注意事項

- `command_task` は `sh -c` で実行されるため、コマンド内容は慎重に管理してください
- `AutoCommit`（自動コミット）と `Rollback`（直前コミットへのリセット）は `loop` および `review` ステップ内で使用されます。動作の詳細:
  - AutoCommit は実行前に作業ツリーの変更をステージしてコミットします。変更が無い場合はコミットを作成しません。
  - 最終リトライで失敗したりユーザーがリジェクトした場合は、そのハッシュに `git reset --hard <HASH>` でロールバックします。
  - リポジトリに初期コミットが存在しない場合は pre-test のハッシュは記録されず、Rollback はスキップされます。
  - Dockerfile では実行環境内で git の user.name / user.email を設定しています。ローカル実行時は `git config user.email "you@example.com"` と `git config user.name "Your Name"` を設定しておくとコミット失敗を回避できます。
- Docker イメージには `gemini` や `copilot` CLI 自体は含まれていません。必要に応じてホストまたはイメージ側で用意してください

## テスト

```bash
go test ./... -v
```

## License

See [LICENSE](LICENSE) file.