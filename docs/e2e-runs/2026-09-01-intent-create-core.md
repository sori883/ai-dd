# Intent作成内部coreの実装検証

- 実施日: 2026-09-01（JST）
- 結果: PASS。内部APIのunit・実filesystem integration・2 process同時作成と、全packageの
  raceを含む検証に成功した。
- Issue: [#31](https://github.com/sori883/ai-dd/issues/31)
- 承認: [Intent作成coreの実装計画](../ram/decisions/2026-09-01-intent-create-core-plan.md)
- source commit: `0c24f2adf0e85e8bd77cf23f79d354f3fe2fb4cd`
- native実行: macOS / arm64、Go `1.26.4`・`1.27.0`

## 実装した契約

`CreateIntent`は既存SpaceへUUIDv7、24文字上限slug、UTC日付と衝突suffixを持つIntentを作成し、
正確なstate stubと既存未知fieldを保持したregistry rowを保存する。registry renameを作成commit
境界とし、その前の失敗はzero result、後のcursor・close・lock release失敗は作成済みmetadataと
errorを返す。

registry read-modify-writeは本家と同じworkspace lock identity、100ms間隔・600 retriesの
owner-stamped directory lockで直列化する。最初のGo版はstale lockを自動回収せずfail closedとする。
shared active-spaceは対象Spaceへ切り替えず、不在時だけ既存fallback値を補完し、対象Spaceの
active-intentだけを作成Intentへ更新する。

このsliceは公開CLIへ接続しない内部coreであるため、配布binaryの利用者向けE2Eとは扱わない。
代わりに実filesystem integrationとhelper subprocessを使い、同時起動した2 processが異なるUUID・
directoryと2 registry rowを失わずに作ることを確認した。

## TDD

実装担当は次の13個のobservable sliceで、変更前の失敗を観測してからGREENへ進めた。

1. UUIDv7のtimestamp・version・variant・entropy failure・uniqueness
2. Intent slugと24文字上限
3. 予約語拒否
4. UTC日付
5. directory collisionと`-999`上限
6. record directoryと正確なstate stub
7. strict registry decode
8. unknown fieldを保持するatomic append
9. mutation readの通常file制約
10. workspace lock path
11. owner stampと自分のgenerationだけのrelease
12. lock下transaction、cursor、commit前後の戻り値
13. JSONの`<>&`を本家と同じくescapeしない保存形式

初回review修正では、target Spaceとshared active-space fallbackの違い、追加処理より先にlockを
解放する順序、U+0130・Final Sigmaを含むWindows identityについて追加REDを観測した。U+1C89を
Unicode 15で変換せずhash prefix `a3f33a77`とするtestは、挙動変更ではない追加時GREENの将来互換
guardとして最初はRED件数へ含めなかった。PR #32のGo `1.27.0` CIではこのguardがU+1C89→U+1C8A、
hash prefix `1e96ce4c`として実際にREDとなった。lowercase 55件、`Cased` 107件、
`Case_Ignorable` 88件、Final Sigma 7文脈へtestを拡張し、Unicode 15 overlay後にGo `1.26.4`・
`1.27.0`の双方でGREENを確認した。raw shell transcriptは永続化しておらず、過去の遷移を独立再現する
証拠とは扱わない。最終GREENはtest sourceとfresh実行で再現できる。

## PR CIで観測したRED

PR作成時の[CI run 33480731964](https://github.com/sori883/ai-dd/actions/runs/33480731964)では、
Go `1.26.x`と6構成buildが成功し、Go `stable`だけが失敗した。`stable`はGo `1.27.0` / Unicode
17.0であり、lock helperがhost toolchainのUnicode tableへ依存していたことが原因だった。
test期待値を緩めず、Bun Windows `1.3.14`のICU 73.2 / Unicode 15へ戻す完全overlayを実装した。
未監査のGo Unicode versionはversion guardで再調査を要求する。

## Fresh検証

repository rootから親がGo `1.26.4`・`1.27.0`の双方で次を実行し、全てPASSした。

| 検証 | 結果 |
| --- | --- |
| `go test -count=1 -shuffle=on ./...` | 両toolchainでPASS |
| `go test -count=1 -race -shuffle=on ./...` | 両toolchainでPASS |
| `go test -tags=integration -count=1 -race -shuffle=on ./...` | 両toolchainでPASS |
| `go vet ./...` | 両toolchainでPASS |
| `go vet -tags=integration ./...` | 両toolchainでPASS |
| `go mod tidy -diff` | 両toolchainでPASS、差分なし |
| `gofmt -l src` | PASS、出力なし |
| 変更Go fileへの`gopls check` | PASS、diagnosticなし |
| `git diff --check` | PASS |

Go `1.26.4`の通常coverageはmain 92.6%、CLI 98.6%、workspace 84.6%、buildinfo 100%、全体87.4%。
profileは`/tmp/ai-dd-issue-31-coverage-go126-final.Goydqk/coverage.out`、SHA-256は
`1edd45977d075c822b5989d422ac00107a8c51b5b1b9fc0df5da8fb3fbd5f6a7`である。Go `1.27.0`は順に
92.6%、98.6%、84.5%、100%、全体87.3%。profileは
`/tmp/ai-dd-issue-31-coverage-go127-final.E47bNy/coverage.out`、SHA-256は
`71d82cd36a91e1955769b2a8083a66e4c04e1828c7d58ef1ad25a33138ca2e3e`である。一時directoryの
長期保持は保証しない。

## 6構成cross compile

最終source commitから`CGO_ENABLED=0`でworkspace integration test binaryをcross compileした。

| OS | amd64 | arm64 |
| --- | --- | --- |
| darwin | PASS | PASS |
| linux | PASS | PASS |
| windows | PASS | PASS |

artifact rootは`/tmp/ai-dd-issue-31-cross-go126-final.9jpuzx/`である。6 fileのSHA-256出力を辞書順に
並べたmanifestのSHA-256は`b615ca1c123c7bce517fca0d1655a60149c0b5858fb208ed8432c1a223d73d88`である。
cross compileは各OSでのnative実行証拠ではなく、Windows Bun `1.3.14`とのlock identityも固定source、
ICU 73.2 / Unicode 15.0 data、全OSで動く既知vectorで検証した。Windows native照合は未実施である。

## 独立review

初回固定head `bf5bd3a`では、shared active-space補完、Windows Unicode lowercase、
caller-held lock primitiveの3件を指摘した。同じ実装担当が`24b9cff`で、shared fallback、
U+0130・Final Sigmaの固定vector、追加処理までlockを保持できるprimitiveへ修正した。

再reviewではローカルmacOS BunのUnicode 17相当system ICUをWindowsへ外挿し、U+1C89を含む
55 mapping差をP1候補とした。Bun `1.3.14`が固定するWebKitとWindows同梱ICUを再調査し、Windowsは
ICU 73.2 / Unicode 15.0、Go `1.26.4`もUnicode 15.0であること、simple lowercase・`Cased`・
`Case_Ignorable`の全範囲比較が一致することを確認した。reviewerはP1を撤回し、source再判定を
No findingsとした。macOS runtime観測はWindows互換表へ取り込まず、誤った推定と訂正理由を
[参照契約](../ram/research/2026-09-01-intent-create-contracts.md)へ残した。

## 本家との差分と限界

ローカル本家2.6.123からの承認済みの意図的な差分は、malformed registryのfail closed、既存Space
必須、commit後cursor失敗の通知、`os.Root`と通常file境界、stale lockを自動回収しないことの5点で
ある。実装・reviewを通じて新しい意図的な差分は追加していない。

rollback、fsync・power-loss耐久、multi-file atomicity、mount/deviceを含む完全sandbox、
Windows/Linuxでのnative実行、未監査の将来Bun/Go Unicode version間互換は保証しない。外部Go module、
public CLI、CI変更は追加していない。
