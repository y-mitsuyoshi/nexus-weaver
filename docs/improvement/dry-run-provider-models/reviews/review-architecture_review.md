次に行う操作: ドキュメントファイルを読み、設計書（GenerateResult 構造体例）が要件（可用性、セキュリティ、整合性、拡張性、保守性）を満たすか厳密にレビューする。ファイルを読み取るためにアーカイブのビューを取得する。（report_intent とファイルビューを並列で実行します）

● Read architecture.md
  │ docs/improvement/dry-run-provider-models/architecture.md
  └ 1 line read

要約：ファイルは「GenerateResult 構造体例（Go）」のプレースホルダのみで、実装・仕様が欠落しています。以下を必須／推奨／軽微指摘として短くまとめる。

MUST（必須修正）
- 完全な構造体定義を追加する（フィールド名、型、jsonタグ、omitempty の有無を明示）。
- schema_version／id／status(enum)／provider(metadata)／created_at/updated_at を含め、後方互換性と整合性を担保する。
- エラー詳細（code/message）とバリデーションルールを明記。
- 機密を含めない設計・出力サイズ上限・ストリーミング方針を定義。

IMO（推奨）
- JSON Schema／protobuf 定義を併記して型安全にする。
- 大容量結果はチャンク化・チェックサムで扱う。
- trace/span id を含めトレーサビリティを確保。

nits（軽微）
- ファイル本文が短すぎるため、例と仕様を分けて記載。ファイルパスは妥当。