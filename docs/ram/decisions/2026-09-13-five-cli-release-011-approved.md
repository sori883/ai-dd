# 五つのCLIと指定版インストーラーを実装する

2026-09-13。五つのCLIを0.1.1へ含める計画と、版選択・配置先・独自ライセンスの
推奨案を提示した後、ユーザーは「はい、実施してください」と回答した。
提示済みの推奨案で実装する直接承認として受け取り、開始時にも採用する3点を明記した。

- `aidlc-install`の`--release-version`で指定したReleaseから、3つの日常用CLIと同版の資材を取得する。
- `project/aidlc/bin/v0.1.1/`へ`aidlc`・`okf`・`natural-japanese-go`を配置する。
- 独自コード・文書はMIT、著作権者`2026 sori883`とする。外部由来の許諾表示は保持する。

[範囲の合意](2026-09-13-five-cli-release-011-scope.md)の「3点が回答待ち」という状態を更新する。
[具体計画](../../design/five-cli-release-011-plan.md)を採用案へ更新し、
単独writerのTDD、独立review、read-only final、GitHub checks、PR mergeを経て
同じ検証済み候補の`v0.1.1`を公開する。旧構成をそのまま公開しない。

Codex向け新規導入、共通原稿の一箇所管理、既存の承認・担当・hook・Git不要の運用を維持する。
Go実装と既存依存を利用し、新しい外部Go moduleは追加しない。
旧配置の互換・移行、Claude・Copilotの再開は含めない。
取得資材の未知schema、版や内容の不一致、既存配置との衝突は拒否する。
公開済みtagや添付を上書きする復旧、利用者の知識やstateの初期化は許可に含めない。

作業先は`/Users/const/sori883/ai-dd-release`、branchは`codex/release-0-1-1`。
開始時にmainとPR #199のmerge commitがともに
`64833d7e94203be0284c18abbf4a35b23810cf77`で、Open Issue・PRは各0件、
同名の0.1.1 Issue・tag・Releaseも存在しないことを確認した。

実装を[Issue #200](https://github.com/sori883/ai-dd/issues/200)として作成し、分類は機能開発とした。
`work_unit_id=five-cli-release-011`の単独writerへ計画の全項目を渡す。

## 完了条件の補足

Issue #200は実装とv0.1.1公開を含む。PRは`Refs #200`で関連付け、merge時に自動closeしない。親がRelease公開と確認を終えた後にIssueをcloseする。GitHub操作は親担当である。
