# 配置移転とUnit再割当の実装証拠

状態: loop完了、親の独立review・final待ち。Issue #138、work_unit_id `clone-relocation`。
開始/終了HEAD `82b711f7cbabfc4b1d9473d156cad7dc7d4e0988`。実装担当によるcommitはない。
[直接承認](2026-09-08-clone-relocation-approved.md)と[実装計画](../../design/clone-relocation-implementation-plan.md)に従った。

## 利用結果と境界

relocateは移転元root/binaryを絶対参照文字列としてだけ検査する。現行Skillの旧/新展開bytesと
5eventの製品handlerを検査し、Skill参照とhook commandのJSON文字列spanだけを更新する。
独自hook/未知属性・書式は保持し、全件検査前に保存しない。専用lock、保存直前bytes比較、
原子的file保存、Paths/Pendingによる部分成功と同要求再試行を実装した。
旧pathへアクセスせず、SKILL原稿と既存WORKFLOWは更新しない。新hook trustは利用者が確認する。

reassignは停止確認・理由・needs_confirmationを必須にし、現在HEAD、base履歴、依存統合、
scopeと他担当を検査する。他Unitの確認待ちruntime不存在はroot/session比較だけを省略し、scopeは維持する。
runtime破損やrunningの割当欠落は拒否する。新run IDを発行し計画/成果を保持する。
state保存失敗時はneeds_confirmationのままでresultを拒否し、同要求は同じrunで保存を再試行する。
現行runtimeだけにreassignment_revision（要求expect）を保持して、同revisionの保存途中と成功後の
後日のpause/resumeを区別する。履歴/stateを追加しない。旧runのresultは拒否する。

## TDDの順序と結果

| slice | 実測したRED / ALREADY_GREEN | GREEN |
| --- | --- | --- |
| install | 許可済み空Relocateで更新pathが0件、不正hooksも受理 | 参照更新、custom bytes保持、重複JSON/UTF-8/末尾/既知属性拒否、symlink/Skill編集拒否、競合・部分失敗再試行 |
| flow | unknown Unit action。空objectの他/自runtime受理も追加回帰RED | 停止確認、同要求retry、後日の新run、旧run拒否、scope/依存、runtime/state障害、既存成果保持 |
| cli | --relocate unknown、root help欠落。通常installへの空source flag混用も追加RED | 厳密flags、reassign、引数/JSON/停止確認/部分成功/trust説明 |
| wiring | relocateを通常installへ送って既存file衝突、同Intent reassignがhookで拒否 | Service接続と既存session/Intent/tool境界。relocateのbootstrap例外は追加しない |
| integration | ALREADY_GREEN（新fixtureで既存sliceの接続を検証） | 2Unitの実CLI引継ぎ→実Go test→result/integrate→新review、元root hash不変と更新後handler実行 |
| docs | 文書更新、人工REDなし | WORKFLOW/help/development/詳細契約へ新操作と利用条件を記載 |

保存競合など追加coverageの初回成功はALREADY_GREENであり、人工REDを作っていない。
テストの旧root/binaryをmacOSの非canonical表記で指定すると既知bytesに一致しなかったため、
installが実際に展開したcanonical表記をfixtureへ渡すよう訂正した。これはINVALID_TEST_FIXTUREであり、
未知参照を許可する製品緩和ではない。

## 末尾検証

以下はすべてexit0。skipや0件testを成功証拠に数えていない。gofmtとgit diff --checkも成功。

```sh
go test -count=1 ./src/internal/install -run '^TestRelocate'
go test -count=1 ./src/internal/flow -run '^TestFlowUnitReassign'
go test -count=1 ./src/internal/cli -run '^TestRelocationCLI'
go test -count=1 ./src/internal/minimal -run '^TestRelocation'
go test -tags=integration -count=1 ./src/cmd/aidlc -run '^TestRelocationCommand'
go test -count=1 ./src/internal/install -run '^(TestInstall|TestFlowInstall)'
go test -count=1 ./src/internal/flow -run '^(TestFlowUnit|TestFlowReview|TestFlowConfigure)'
go test -count=1 ./src/internal/cli -run '^(TestRun|TestMemoryHelp|TestConfigureHelp|TestMinimal|TestFlow)'
go test -count=1 ./src/internal/minimal -run '^(TestHook|TestSession|TestFlow|TestMemoryHelp)'
```

末尾ログは `/tmp/relocation-boundary.log`。各有効REDは `/tmp/relocate-*-red.log` に保持した。
全package/race/vet/build gate/長時間live/GitHub操作は実施していない。
SKILL原稿・ユーザーAGENTS・参照資料・他worktreeを保全した。外部module/tool追加はない。

## 限定liveの親final入口

```sh
AIDLC_RELOCATION_LIVE=1 go test -tags=integration -v -count=1 -timeout=15m ./src/cmd/aidlc -run '^TestRelocationLive$'
```

このloopでは未実行。固定Codex 0.153.4/gpt-6-astra medium、workspace-write/never、既存のtest trust方式を使用する。
HOME/CODEX_HOME/認証設定を変更しない。移転済hooksの実commandを検査・保存してからtest relayを挟み、
既存の製品hookを同じ新root/binaryで実行する。raw hookの同一Intent ID選択Pre/Postと最終sessionを照合し、
実transport exit・本文と保存hash・metadataによるKnowledge作成/更新を確認する。元rootの非.gitファイルhashも比較する。
自己申告・timeout・skipは成功扱いしない。2Unitの完走は実CLI/実Go testのfixtureであり、実AI worker完走とは区別する。

## 本家との関係

固定AI-DLC 2.6.123の通常配置はcopy中心であり、同名relocate/reassign契約は確認されていない。
この2操作は直接承認されたGo四段階製品の追加機能であり、本家install/updateと同等とは主張しない。
利用者編集資産とGit共有正本を保持し、旧runtimeを共有せず、停止確認と新hook信頼確認を利用条件とする。
以前の日常運用検証に記録した制約は削除せず、この後続の明示操作で対応した。
