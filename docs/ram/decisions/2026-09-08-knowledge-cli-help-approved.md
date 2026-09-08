# CLIによるfrontmatter生成とhelp参照を実装する

状態: Accepted。ユーザーは前案に「はい、お願いします」と回答し、CLIが取れる型・status等の選択肢をhelpへ、
Skillには困ったらhelpを参照する手順を入れるよう直接依頼した。
[前案](2026-09-08-cli-owned-knowledge-frontmatter.md)の作成・更新方式を採用する。
旧方式のAIによるfrontmatter付き草稿から、引数metadataと本文だけの草稿に変更する。

[実装計画](../../design/knowledge-cli-frontmatter-plan.md)に引数、日時、保持、help、検証範囲を具体化した。
生成日時は自動、出典・検証・期限の日時は意味を保つ。typeは自由文字列、statusは既存OKFの3値を維持する。
helpは正常な読取りとして未選択でも使え、型の一覧はhelpへ集約する。
入力経路の置換とmetadataのみの明示更新を含む。新module・state遷移変更・旧データ移行は含まない。
前の四段階実装の包括承認ではなく、今回の直接承認で進める。
