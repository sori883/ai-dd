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
