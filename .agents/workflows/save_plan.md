---
description: AntigravityのPlanモード終了後に、生成された計画ファイルを適切に命名して保存する手順
---

# Planモード成果物の保存ワークフロー

Antigravity（AI）がPlanモードで `implementation_plan.md` などの計画ファイルを作成した後、このワークフローに従ってリポジトリ内に成果物を保存します。

1. **ファイル内容の確認と命名**
   - 成果物ディレクトリ（Artifact Directory）に生成された `implementation_plan.md` の内容を確認します。
   - その内容を簡潔に表す適切な英語のファイル名を決定します。
     - 良い例: `api_endpoints_plan.md`, `docker_tls_fix_plan.md`
     - 悪い例: `implementation_plan.md`, `plan.md`

2. **ファイルのコピーとリネーム**
   - 決定したファイル名を使用して、`docs/plans/` に計画ファイルをコピーします。
// turbo
   - 実行例: `cp <成果物のimplementation_plan.mdのパス> docs/plans/決定したファイル名.md`

3. **.gitignore の確認（初回のみ）**
   - `docs/plans/` ディレクトリが `.gitignore` で無視されているか確認し、されていなければ追加します。

4. **保存完了の報告**
   - ユーザーに対して「計画ファイルを `docs/plans/〇〇_plan.md` として保存しました」と報告します。
