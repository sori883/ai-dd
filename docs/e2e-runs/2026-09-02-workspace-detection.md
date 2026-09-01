# 読み取り専用ワークスペース分析の実装検証

- 実施日: 2026-09-02（JST）
- 結果: PASS。内部APIのunit・実filesystem integrationと、全packageのraceを含む検証に成功した。
- Issue: [#33](https://github.com/sori883/ai-dd/issues/33)
- 承認: [読み取り専用ワークスペース分析の実装計画](../ram/decisions/2026-09-02-workspace-detection-plan.md)
- source commit: `9d428b1b688f00a69c823b6cbe47d589ca706f4e`
- native実行: macOS / arm64、Go `1.26.4`

## 実装した契約

callerがlifecycleを所有する`*os.Root`から、project type、languages、frameworks、build system、
nested root、Git submoduleをread-onlyで算出する`workspace.Detect`を実装した。root signal、nested depth 3、
JavaScript UTF-16順、native language tie、framework・build priority、weakly typed `package.json`、
`.gitmodules`の宣言順と安全pathは、ローカル本家AI-DLC `2.6.123`の実装を基準にした。

公開CLIへ接続しない内部sliceのため、配布binaryの利用者向けE2Eとは扱わない。実filesystem integrationで
known result、tree無変更、symlink境界、内部I/O errorの吸収を確認した。

## TDD

実装担当は次の7個のobservable sliceについて、変更前の失敗を観測してからGREENへ進めたと報告した。

1. rootのGreenfield / Brownfield分類
2. language検出、深さ、閾値、native tie
3. weakly typed `package.json`
4. framework固定順とbuild priority
5. nested workspace探索
6. `.gitmodules` parser
7. submodule結果とnested後の分類

各sliceのnarrow commandは`go test -count=1 ./src/internal/workspace -run '^<TestName>$'`である。
当初7 sliceのRED / GREEN raw logは保存されていないため、過去の遷移を独立再生できる証拠とは扱わない。
I/O failureと実filesystem integrationは追加時点GREENのguardであり、RED件数へ含めていない。最終GREENは
test sourceとfresh実行で再現できる。

初回独立reviewでは、有効JSONの`1e400`をGoの`json.Unmarshal`がfloat64 overflowとして文書ごと
拒否し、本家のJavaScript `JSON.parse`によるInfinity / truthy評価とずれるP1を指摘した。
overflowする正負のdependencies / peerDependenciesを追加してREDを観測し、`Decoder.UseNumber`と
`json.Number`のJavaScript相当truthinessへ修正後にGREENを確認した。underflow、`0`、`-0`、trailing
JSON tokenは追加時GREENの境界guardである。

P1修正のRED / GREEN raw logは`/tmp/ai-dd-issue-33-review-fix.log`、SHA-256は
`f69dd75c17c165b385488aa80ee06b8cfc7c3ccf581343517601eb0becb00108`である。一時fileの長期保持は
保証しない。

## Fresh検証

最終source commitについて、実装担当、親、独立review担当が役割に応じて次をfresh実行し、すべてPASSした。

| 検証 | 結果 |
| --- | --- |
| 対象unit test | PASS |
| 対象integration test | PASS |
| `go test -count=1 -shuffle=on ./...` | PASS |
| `go test -count=1 -race -shuffle=on ./...` | PASS |
| `go test -tags=integration -count=1 -race -shuffle=on ./...` | PASS |
| `go vet ./...` | PASS |
| `go vet -tags=integration ./...` | PASS |
| `go mod tidy -diff` | PASS、差分なし |
| `gofmt -l` | PASS、出力なし |
| 変更Go fileへの`gopls check` | PASS、diagnosticなし |
| `git diff --check` | PASS |

## 6構成cross compile

最終source commitから`CGO_ENABLED=0`でworkspace integration test binaryをcross compileした。

| OS | amd64 | arm64 |
| --- | --- | --- |
| darwin | PASS | PASS |
| linux | PASS | PASS |
| windows | PASS | PASS |

artifact rootは`/tmp/ai-dd-issue-33-cross-final.EY71SG/`である。6 fileのSHA-256出力を辞書順に並べた
manifestのSHA-256は`9ee97c7817770f04cfe726f860622bdae8c738df6f69d327043fbd8f7517f5d5`である。
cross compileは各OSでのnative実行証拠ではなく、Windows実機のsymlink・filesystem動作は未検証である。

## 独立review

初回固定head `a8b0c86`では上記のJSON number overflowをP1として指摘した。同じ実装担当が
回帰testを先行して追加し、commit `9d428b1`で修正した。修正後の固定base
`33e78a54b2a92f89ce6802178d44431974f23032`からhead
`9d428b1b688f00a69c823b6cbe47d589ca706f4e`を再reviewし、元P1の解消を確認した。追加findingはなく、
独立実行した対象test、race、vet、tidy、gofmt、gopls、diff checkもPASSした。

## 本家との差分と限界

本家2.6.123からの意図的な差分は、既存のproject安全境界を継承して`os.Root`によりroot外・absolute
symlinkを拒否する点である。本家の通常filesystem APIが追従し得るtreeでは、Go版はsignalを検出しない。
実装・reviewを通じて新しい意図的な差分は追加していない。

同数languageの順序はnative directory列挙順に依存する。複数回のreadを一貫したsnapshot下で行わないため、
並行filesystem変更中の完全な決定性は保証しない。`os.Root`はmount / deviceを含む完全sandboxではない。
外部Go moduleは追加していない。
