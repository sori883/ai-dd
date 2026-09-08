# Knowledge CLI metadataとhelpの実装証拠

状態: loop完了、独立reviewと親final待ち。Issue #132、work_unit_id `knowledge-cli-frontmatter`。
開始HEADは `346d897b2c4aa95825b727ad637a6e0cccc92a18`。実装担当によるcommitはない。
直接承認と入力契約は [承認記録](2026-09-08-knowledge-cli-help-approved.md) と
[自己完結計画](../../design/knowledge-cli-frontmatter-plan.md) に従った。

## 結果と境界

memory create/updateは本文のみの `--body-file` を読み、CLIがmetadataとgeneratedを生成する。
更新は省略した既存の任意項目・未知拡張を保持し、明示metadataのみの変更も保存する。
actor変更や時刻更新だけを内容変更とは数えない。metadata-jsonは指定した最上位キーだけを置換し、
その値がobjectの場合はそのobject全体を明示値とする。既存OKFで省略可能なtitle/descriptionを持たない
文書も更新可能であり、作成時だけ新しい必須入力条件を適用する。
CAS、部分保存診断、Space隔離、path/symlink/容量境界は維持する。外部依存追加はない。

helpは同じ実行binaryの厳密な文法だけをhookで許可し、session/Rules/slotを変更しない。
別binary・redirect・compound・書込み引数混在の例外許可はしない。
配置SKILLは2398 byteで4 KiB内。型と値の正本はhelpとし、Skillはhelp参照を案内する。
旧memory --fileの製品例はbody-fileへ更新し、Intent/Unit --fileは保持した。

## 順序付きTDD

| 項目 | 実装前の有効なRED | GREENで確認した結果 |
| --- | --- | --- |
| C1 TestMemoryMetadataCLI | --body-fileがunknown flag | 必須・省略presence・繰返しtag・clear・旧flag拒否、Intent/Unit入力維持 |
| C2 TestMetadataInput | 許可済み空builder scaffoldが文書を返さない | YAML quoting、未知項目保持、生成時刻、厳密JSON、metadata-only/no-op |
| C3 TestMemoryBodyWrite / TestSessionMemory | 旧File参照でinvalid relative path | 本文のみ保存、CAS、metadata保持、partial failure、Space境界 |
| C4 TestMemoryHelp | help未対応と未選択hook拒否 | root/group/actionのhelp、callback非実行、hook状態不変、安全拒否 |
| C5 TestInstallMemory / TestMemoryHelp / TestMemoryMetadataCommand | 旧配置文法とnested cwd本文path不解決 | 実配布内容、4KiB、実binaryでKnowledge/ADR/Rule作成更新検索 |

追加回帰で不正UTF-8 JSONをdecoderが置換して受理する問題と、既存文書の任意項目を新規必須として
要求する問題をそれぞれ有効REDから修正した。限定live証拠validatorも未実装時の偽成功を拒否する
fixture REDから、実command結果・Pre/Post・文書bytes・本文bytes・会話対応を拘束した。
helper追加中のunused importによるcompile failureはREDに数えない。
既存のhelpを不正入力とするfixtureは契約変更による期待値訂正であり製品REDとは数えない。
閉pipeテストは不正文法を `-h` に置換してexit/保持境界を維持した。

## 境界検証

次の全コマンドはexit 0。0件testの結果はない。gofmt適用とgit diff --checkも完了。

```sh
go test -count=1 ./src/internal/cli -run '^TestMemoryMetadataCLI'
go test -count=1 ./src/internal/okfmemory -run '^TestMetadataInput'
go test -count=1 ./src/internal/minimal -run '^(TestMemoryBodyWrite|TestSessionMemory)'
go test -count=1 ./src/internal/minimal ./src/internal/cli -run '^TestMemoryHelp'
go test -count=1 ./src/internal/install -run '^(TestInstallMemory|TestMemoryHelp)'
go test -tags=integration -count=1 ./src/cmd/aidlc -run '^TestMemoryMetadataCommand'
go test -count=1 ./src/internal/cli -run '^(TestRun|TestMinimal|TestFlow)'
go test -count=1 ./src/internal/minimal -run '^(TestFlow|TestHook|TestSession|TestRules)'
go test -count=1 ./src/internal/install -run '^(TestInstall|TestFlowInstall)'
go test -count=1 ./src/cmd/aidlc -run '^(TestFlowCommand|TestSpaceCreator|TestSpaceLister|TestSpaceSwitcher|TestMainSpace|TestMainRoot)'
```

## 親finalの限定live入口

```sh
AIDLC_MEMORY_LIVE=1 go test -tags=integration -v -count=1 -timeout=15m ./src/cmd/aidlc -run '^TestMemoryMetadataLive$'
```

このloopでは実行していない。Codex CLI 0.153.4、gpt-6-astra/medium、workspace-write、neverを維持する。
HOME/CODEX_HOMEや認証は変更せず、test用trust mapと検査済みrelay hookだけを使用する。
一つの実会話で未選択help、選択/Rules、本文のみKnowledge作成・更新を観測する。
raw commandのexit/出力、同じhook IDのPre/Post、保存文書hash、本文実体、保持metadataを照合する。
自己申告・skip・timeoutを成功扱いしない。証拠は表示されるtemp directoryへ保持する。
全project/race/vet/cross-build/四段階liveはこのloopでは実行していない。

## Issue #132 独立review修正

P1: unit confirmのJSON説明からcommitが欠落していた。既存flow実装はresultとconfirmの双方で
現在worker HEADとの一致と40桁を要求するため、helpと詳細契約を訂正した。新しい挙動は追加していない。
`TestMemoryHelpUnitConfirmCommit` を先に追加し、必須fieldとHEAD/40桁/必須の説明欠落で
runnable RED（exit 1）を確認後、説明を修正してGREEN。
境界: `go test -count=1 ./src/internal/cli -run '^TestMemoryHelp'` 成功、gofmtとdiff check成功。
