# nexus-weaver

🤖 An autonomous, terminal-based SDLC orchestrator. Powered by multi-LLM workflows (Gemini / Local Qwen) and self-healing Test-and-Fix loops in Go.

## Overview

nexus-weaver は、YAML駆動ワークフローで複数のLLMエージェントを協調させ、PRD作成→設計→実装→テスト→PR作成までの開発ライフサイクルを自律的に実行するCLIオーケストレーターです。

## Directory Structure

```
nexus-weaver/
├── cmd/nexus-weaver/     # CLI エントリーポイント
├── internal/
│   ├── engine/           # ワークフローパーサー & 実行エンジン
│   ├── llm/              # LLMProvider インターフェース & アダプター
│   └── fs/               # ファイル I/O & Git ユーティリティ
├── workflows/            # パイプライン定義 YAML
├── prompts/              # エージェントプロンプトテンプレート
└── docs/                 # PRD & アーキテクチャ設計書
```

## Quick Start

### 1. Run natively with Go

```bash
# ビルド
go build -o nexus-weaver ./cmd/nexus-weaver

# ドライラン（ワークフロー読み込みのみ）
./nexus-weaver --workflow workflows/workflow.yml --dry-run
```

### 2. Run with Docker Compose

Dockerを使えば依存環境（Goのインストールなど）なしで実行できます。ホストのカレントディレクトリをマウントするため、ファイル生成やGitコミットもホスト側に反映されます。

```bash
# ビルド & 実行 (ドライラン)
docker compose run --rm nexus-weaver --dry-run

# 本実行
docker compose run --rm nexus-weaver
```

## CLI Options

| Flag | Default | Description |
|------|---------|-------------|
| `--workflow` | `workflows/workflow.yml` | ワークフロー定義YAMLファイルのパス |
| `--verbose` | `false` | 詳細ログの出力 |
| `--dry-run` | `false` | ワークフロー検証のみ（実行しない） |

## Core Features

- **YAML駆動ワークフロー**: ステップ、役割、モデル、入出力をYAMLで定義
- **マルチLLMルーティング**: ステップごとに Gemini CLI / Local Qwen を切り替え可能
- **自律的 Test & Fix ループ**: テスト失敗時にエラーをLLMに渡して自動修正
- **安全装置（Auto-Rollback）**: テスト実行前にGit自動コミット、暴走時のロールバック対応

## License

See [LICENSE](LICENSE) file.
