# 6区切りのテスト削減結果

ユーザーの「削除」「区切って対応」「Issue作ってmrでマージ」という直接依頼に従い、[全体計画](../../design/test-reduction-milestones.md)の151候補を6区切りで処置した。候補はテスト件数ではなく、削除・統合・準備の共有・理由付き保持を判断する単位である。各候補の実害、残る保証、検証入口はM1〜M6の結果表へ記録した。

- M1〜M5はIssue #204/#206/#208/#210/#212とPR #205/#207/#209/#211/#213を完了し、main反映後の検査を確認した。
- M6はIssue #214でCIの重複実行を整理した。単独writerの結果は[M6結果](../../design/test-reduction-m6-result.md)。独立レビュー・親final・同headのGitHub実行・mergeの証拠は、[PR #215](https://github.com/sori883/ai-dd/pull/215)本文と状態へ記録する。

基準commit `adc4682ca265c9bda4434a94d9d99cff1f4b9394` とM5 merge `d8aa6cf3415e3cfe984dc2f530bd9da398cdddc2` の `src/**/*_test.go` を比較すると、216ファイル・35,850行から192ファイル・29,464行になった。差は24ファイル・6,386行。M6はGo/testを変更しない。この数字は作業量の説明であり、削減目標や速度のbenchmarkではない。

共有できる不変binaryだけを共有し、可変state・作業root・証拠は分離した。保存失敗、承認や担当の不一致、内容SHA、Unicode/CLI出力、license、異なるOSとPowerShellの固有保証は維持した。容量2testとJSONの公開順序、tar/zipなど、削除すると独自の不具合を見逃す検査は理由を付けて保持した。

CIはPRとmain pushへ集約し、通常/ファイル操作testはGo 1.26.xとstable、race等は主要版、5CLI×6環境buildは固定版Distributionが担当する。3OSで同じ配布候補を起動し、WindowsではPowerShell5.1/7を確認する。実機Codexの調査は明示した診断手順へ分離し、記録未設定によるskipを実機成功とは扱わない。

M1/M2で観測したWindowsの一時的なdirectory移動失敗は[別記録](../research/2026-09-14-windows-native-rename-observation.md)へ残した。以降の成功だけで原因が解消したとは判断しない。Release/tagの公開は行わない。
