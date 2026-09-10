# 配布の仕上げを本家Codexの手順に沿う最小構成で進める

- 状態: 順次対応の直接依頼を具体化した実装範囲。公開の承認ではない。
- 基準: PR #164 merge後のmain `c5ec2e9f0243cf7382a90d4d65d45929dd5227ca`。
- 実装管理: [Issue #165](https://github.com/sori883/ai-dd/issues/165)、開発運用の整備としてユーザーリクエスト分類。

ユーザーの実案件→配布・更新→利用者文書を順に進める依頼に従い、2番目を具体化した。
実案件は同じIntentの6段階を完了し、独立review、read-only final、GitHubの16checksが成功した。
PR #164のmain反映とIssue #163のcloseを確認した。通常test初回はmodule内に保全したTDDの.goコピーを
拾ってsetup failedになったため、その証拠を保持し、同じ834fileのbytes/modeとHEADを持つ検証checkoutで
計画全体を実行した。製品codeを変更した補正ではない。

配布計画は [具体計画](../../design/distribution-packaging-plan.md) にまとめた。
Go単一binaryを6target向けarchiveへ梱包し、manifestとSHA256SUMSを生成する。local/CI内で展開・導入を
確認し、既設の比較・更新・復旧を利用者データ保全の手順として整える。
新しい製品updater、操作台帳、receipt、旧Intent移行は追加しない。
plannerが最初に出した自動apply/rollback案は未採用の検討案であり、承認済み要件へ持ち込まない。

## 固定本家の追加確認

元checkoutの `docs/実装_aidlc-workflows/`、同 `dist/codex/`、`docs/配布_ai-dlc/` を読取り専用で確認した。
3箇所のversionは2.6.123で、aidlc-utility.tsのSHA256も一致した。

- `docs/guide/harnesses/codex-cli.md:37`: 配布treeのコピー、設定merge、Git、通常hook trust、doctor。
- `core/tools/aidlc-utility.ts:6042`: handleUpgradeは未提供のエラーへ到達する。
- `scripts/package.ts:936`: checkHarnessは生成物とcanonical distの比較。利用先編集の識別ではない。
- `docs/reference/06-hooks-and-tools.md:222`: 停止後、対応するengine/library/hooks/生成物を組で更新・復旧する。
- `harness/cursor/install.ts:1001,1147`: receiptを使った編集検出と順次書込みはCursor固有であり、Codexの
  既存契約へ流用しない。全体transactionやrollbackの保証とも同一視しない。

最初の専用cloneにはignored snapshotがなかったが、元checkoutでの追加読取りにより上記を確認できた。
最新upstream全体の確認とは区別する。Go単一binary化は既承認の差分を維持する。

## 実装と公開の境界

開発用梱包command、新CI、限定運用文書は直接依頼の範囲内として計画・Issue後に実装する。
正式version、Go製品のライセンス、実tag/Release/配布物uploadは未確定で、今回自動的に決めない。
CI内の候補検証にも公開artifact uploadを含めない。新しい外部Go moduleやtool、CIの追加権限は不要。
GitHubの3OS標準runnerは[公式資料](https://docs.github.com/en/actions/reference/runners/github-hosted-runners)を
Context7と公式webで確認した。cross-buildとnative実行、配置生成と実Codex hook実行は別の証拠として扱う。
Windows側の実hook経路は未確認。既知のlinked-worktree hook探索問題も修正済みとしない。

元checkoutの未commit資料、完了pilotのstate/Knowledge/runtimeは変更せず、専用worktreeで単独writerが実装する。
利用先のRule、Knowledge、Intent/state/history、registry/runtimeを削除・初期化して更新成功へ見せかけない。

## 実装loopの証拠（2026-09-11）

Issue #165、work_unit_id `distribution-packaging`、開始HEAD
`167846947f1290d5eeab10056e6735206b25d22f`。単独writerで梱包command、CI、手順を追加。
標準ライブラリのみを使用し、製品CLIと依存関係は変更していない。

各commandは `go test -count=1 ./src/cmd/aidlc-dist -run '<pattern>'`、
cwdは `/Users/const/sori883/ai-dd-distribution`。testを先行して追加した。

| pattern | RED | GREEN | 観測 |
| --- | --- | --- | --- |
| `^TestArchiveLayout$` | exit 1 | exit 0 | 6archive欠落から、形式・内容・Unix modeを確認 |
| `^TestArchiveReproducibility$` | exit 1 | exit 0 | manifest欠落から、8成果物の再現性・hash・size・順序を確認 |
| `^TestArchiveRejectsInvalidInput$` | exit 1 | exit 0 | 不正入力の受理から、出力前拒否・保存失敗を確認 |
| `^TestDistCommand$` | exit 1 | exit 0 | 空scaffold応答から、help・対象選択・exit区別を確認 |

既存出力dir拒否はslice 3時点でALREADY_GREEN。人工REDは作っていない。
ZIPのゼロ時刻が1979年として読まれる追加回帰もexit 1を確認後、
1980-01-01 UTC固定に修正してexit 0。末尾4targetedおよび
`go test -count=1 ./src/cmd/aidlc-dist` はすべてexit 0。gofmtとdiff checkも完了。

証拠root: `/var/folders/9w/921pjkys39q28sk4xsc0hs000000gn/T/ai-dd-dist-tdd-r5lp3hy0`。
`01-layout-*`、`02-repro-*`、`03-invalid-*`、`04-command-*`、
`01-layout-timestamp-*`、`boundary-*` にcommand/cwd/time/exit/stdout/stderr、
test・productionのSHA256とmodule外snapshotを保存した。

finalの6target実build後の入口:
`AIDLC_DIST_DIR=<候補dir> go test -tags=integration -count=1 -v ./src/cmd/aidlc-dist -run '^TestDistributionArchives$'`。
env未指定skipは検証成功ではない。native手順はenv不要の
`go test -tags=integration -count=1 -v ./src/cmd/aidlc-dist -run '^TestDistributionJourney$'`。
同じsourceから版表示・pathの異なる2binaryを作り、手動切替・参照補正・独自hook/data保全・
元bytes復元を検査するfixtureであり、未知版upgrade互換の証明ではない。
loopではintegration/E2E/6build/全project/race/vet未実行。3OS CIはfinalへ残す。
Windowsの実Codex hook実行は未確認。

## native fixtureのbinary参照修正

work_unit_id `distribution-native-path-repair`。固定HEAD `de0f2e6` の親finalでは
TestDistributionJourneyがrelocateのunknown product commandで失敗した。
ログは `/var/folders/9w/921pjkys39q28sk4xsc0hs000000gn/T/ai-dd-distribution-final-087dtus1/distribution-journey-stdout.txt`。
製品の `src/cmd/aidlc/minimal.go` は実行binaryをEvalSymlinksして配置する一方、fixtureは
`/var/...` の未正規化pathを返し、`/private/var/...` が埋め込まれたhookと一致しなかった。
relocateの旧参照は文字列契約で、実在不要のため製品側で正規化する修正は行わない。

fixtureのbinary返却処理を小helperへ抽出し、symlink経由の参照が実pathになる回帰を先行。
`go test -tags=integration -count=1 ./src/cmd/aidlc-dist -run '^TestDistributionBinaryPath$'`
は旧返却処理でexit 1、EvalSymlinks適用後exit 0。証拠は既述TDD rootの
`binary-path-red` / `binary-path-green`。製品code・拒否条件は変更していない。
gofmt/diff checkを実施。full Journey/全体/race/vet/crossbuildはloopでは再実行せず、
修正後の親finalで確認する。以前のfinal成功項目はこの差分の成功証拠には流用しない。
