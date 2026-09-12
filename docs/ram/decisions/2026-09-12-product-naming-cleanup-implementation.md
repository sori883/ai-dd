# 製品名称整理の実装

日付: 2026-09-12。状態: Issue #173のloop実装を完了、独立review・final・mergeは親が確認する。

[直接承認](2026-09-12-product-naming-cleanup-approved.md)と
[実装計画](../../design/product-naming-cleanup-plan.md)に従い、core/Codex原稿を各直下へ、
内部接続を `src/internal/app` へ移した。CLIの入力は `CommandRequest`、解析は `ParseCommand`、
実行接続は `Dependencies.Execute`、実行入口は `executeCommand` とした。

新しい内部hook commandは `__hook`。CLIによる新名受理・旧名拒否と一度だけのdispatch、
5イベントの新名生成、移転時の既知handler補正をtest先行で確認した。
実行入口のSessionStart bootstrapも新名で検査した。未知の製品handler拒否、独自handler保持、
部分失敗・再試行の既存契約を保持する。旧名の別名・移行は提供しない。

文書・TOML原稿のbytesと利用先mappingを維持し、stage・Sensor・承認・予約・保存schemaは変更していない。
旧計画や過去の検証記録は履歴として保存した。一般的な最小Ruleを意味するtest名も保持した。

work unit `product-naming-cleanup` は既存対象testをALREADY_GREENとして開始した。
CLIの `TestHookCommandDispatch` は新名の拒否・旧名の受理をREDとして観測した。
`TestInstallHookCommands` と `TestRelocateReferences` は新名の生成・照合不一致でREDを観測した。
`TestMainHookCommand` は新入口が通常処理へ送られるREDからbootstrapのGREENへ進めた。
構造移動は故意のREDを作らず、既存の限定testを維持した。

検証対象は計画のCLI・install・app・workspace・cmd/aidlcのtargeted command群。
全package・race・vet・cross build・配布E2E・実Codexはloopでは実行していない。
この記録だけを配布や実機の成功証拠として扱わない。
