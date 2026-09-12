# 製品構成から旧minimal名称を取り除く計画

日付: 2026-09-12。状態: ユーザーの直接依頼で実装する範囲を具体化。

## 背景・目的・実装許可

現在のAI-DLCは一つの製品として配布されるが、最小構成へ転換した時期の名前である
`minimal`が、原稿フォルダ、Goのpackage、CLIの内部接続、hookの呼出し名に残っている。
別の通常版と最小版があるように見えるため、製品資材を本来の配置へまとめ、内部名も役割で表す。
hookはCodexの操作前後にCLIを呼び、Intentや承認待ち等の条件を確認する仕組みを指す。

ユーザーは `src/core/minimal`、`src/harness/codex/minimal` の名前を不要と指定し、
`src/internal/minimal` と `__minimal-hook` も改名対象に含める確認に
「はい。お願いしたいです。」と回答した。これを本計画の直接実装許可とする。
旧33 Stageの包括承認は使用しない。旧記録の後方互換・移行は不要という既存合意を維持する。

開始時点のmainは `2cab52b52cf38338c682299e5e6a4e0788e491c4`、直前の実装はmerge済みPR #172。
Open Issue・PRは各0件。元checkoutの未commit資料・agent編集は保全し、
`/Users/const/sori883/ai-dd-naming`、branch `codex/remove-minimal-naming` で作業する。
この開発用worktreeは利用者にworktreeを要求するものではない。

## 変更内容

| 現在 | 変更後 | 理由 |
| --- | --- | --- |
| `src/core/minimal/*` | `src/core/*`、package `core` | 共通の初期文書原稿をcore直下へ置く |
| `src/harness/codex/minimal/*` | `src/harness/codex/*`、package `codex` | Codex用Skill・agent原稿をCodexの配置へまとめる |
| `src/internal/minimal/` | `src/internal/app/`、package `app` | CLIとhookを接続するアプリケーション処理を表す |
| `src/internal/cli/minimal.go` / `_test.go` | `command.go` / `command_test.go` | 共通commandの解析・呼出しを表す |
| `MinimalRequest` / `ParseMinimal` | `CommandRequest` / `ParseCommand` | 入力型と解析関数の役割を表す |
| `Dependencies.Minimal` | `Dependencies.Execute` | 実処理を呼び出す接続点を表す |
| `src/cmd/aidlc/minimal.go` / `minimalCommand` | `command.go` / `executeCommand` | 実行入口を表す |
| `__minimal-hook` | `__hook` | hook専用の内部command名を短くする |

CLI内の `isMinimal` / `runMinimal` は `isServiceCommand` / `runServiceCommand` とする。
移転先に同名file・型の衝突はない。`runtime`はGo標準packageと混同するため内部名に採用しない。
packageを細分化する設計変更は行わず、現在の依存関係を保って移動する。

テスト補助関数・fixture・環境変数も製品旧名称に由来するものを改名する。
例: `buildMinimalBinary` → `buildAIDLCBinary`、`minimal_hook_probe*` → `hook_probe*`、
`AIDLC_MINIMAL_HOOK_LIVE/EVIDENCE` → `AIDLC_HOOK_LIVE/EVIDENCE`、
`space_minimal_test.go` → `space_okf_test.go`。既存名との衝突時は役割の分かる名前へ調整する。
過去のRAM・計画・検証証拠、固定参照資料、一般的な「最小」の意味の用語を一括置換しない。

## 維持する契約と配置への影響

埋込み資材の相対名と利用先の配置を保つ。`aidlc/spaces/<space>/knowledge/`、
`aidlc/workflow/`、`.agents/skills/`、`.codex/agents/` は同じ場所へ生成する。
文書本文、担当一覧、stage、Sensor、承認条件、予約の排他・明示解放を改名のために変更しない。
Intentやsession等の保存形式・schema番号、検索範囲、通常CLI操作も変えない。
Go単一バイナリ・標準ライブラリを維持し、外部module/toolは追加しない。

新規配置の5イベントは `__hook` を呼ぶ。`--relocate` の既知command照合と不明commandの拒否も
新名に揃える。旧名の別名対応や自動変換は追加しない。新binaryだけの交換では旧hook設定が動かないため、
binaryとそれに対応する製品handlerを組で扱う必要がある。現行版で生成したstaging資材を比較し、
独自hookを保ったうえで選択したhandlerの参照だけを補正する既存手順に、この条件を明記する。
旧handlerを残した配置へ新binaryで `--relocate` を実行しても版移行にはならない。
この作業では利用者の配置・runtime・元checkoutを自動更新しない。
hook変更後はCodexの通常の信頼確認が必要で、trust設定や検査を迂回しない。

本家との新しい工程・権限制御の仕様差分は採用しない。今回確認するのはmainのGo配置・内部接続であり、
固定本家2.6.123や最新upstream全体との同一性を新たに保証するものではない。

## ファイル所有権と順序

親が計画・承認RAM・索引・Issueを用意した後、単独の `go_tdd_implementer` が
work unit `product-naming-cleanup`、`verification_mode=loop` で全実装項目を担当する。
実装中は親や他agentが同じworktreeを編集しない。実装担当はsubagent・Issue・PRを操作しない。

所有範囲は移動元/先の3package、`src/internal/cli/`、`src/cmd/aidlc/` の直接参照とtest、
`src/internal/install/{install,relocate}.go` と関連test、
`src/internal/workspace/space_create.go` と関連test、
`src/internal/flow/boundary_sensor_test.go`、
`src/cmd/aidlc-dist/distribution_integration_test.go`、
現行の `docs/{architecture,development,distribution}.md`。
必要な実装証拠RAMと索引更新も単独writerへ渡す。

1. 変更前の既存対象testを確認する。純粋な移動は `ALREADY_GREEN` と記録し、故意に失敗させない。
2. 新名 `__hook` をCLIが受理し実処理へ一度だけ渡すtestを先に追加し、実行可能なREDを確認する。
   旧名を受理しないこと、未知引数の拒否も確認し、最小修正でGREENにする。
3. fresh installで5イベントに新名が入り、移転後も新名で既知参照だけを補正するtestを先に追加する。
   新名へのRED後、生成・照合を更新する。独自handler・既存file・部分失敗・retryの検査を維持する。
4. package・原稿・識別子・fixtureを移動し、importと直接参照を更新する。
   埋込み文書の内容と配置を既存testで確認し、hook拒否・Rule読込・session復旧の回帰を実行する。
5. 現行文書、開発者向けprobe環境変数・test名を追従する。gofmtはloop内で完了する。

targetedは `go test -count=1` に限定し、以下の対象を使う。

```sh
go test -count=1 ./src/internal/cli -run 'Test(Command|Minimal|Hook|Flow|Assignment)'
go test -count=1 ./src/internal/install -run 'Test(Install|ProductAgent|Relocate)'
go test -count=1 ./src/internal/app -run 'Test(Hook|Session|Rules|Agent|Child|Flow|Assignment)'
go test -count=1 ./src/internal/workspace -run '^TestCreateSpace'
go test -count=1 ./src/cmd/aidlc -run 'Test(HookProbe|FlowCommand|Main)'
```

移動前はappをminimalへ読み替える。新規testの正確な名前は実装証拠へ記録する。
tag付きfixtureはloopで実行せず、必要なcompile確認だけ `-tags=integration -run '^$'` を使う。

## 受入条件・review・final

旧フォルダと旧製品識別子が現行sourceからなくなり、新名でbuildできる。
旧名拒否testに必要な文字列だけは残してよい。新規Spaceの文書、5担当、2Skillが同じ利用先へ生成される。
新しいhook入口で許可/拒否が働き、配置移転が独自設定を壊さない。旧資材からの版移行を保証しない。

親が全差分とtargeted群を作業単位末尾で一度確認し、独立reviewerが固定base/headを
`verification_mode=review` で読取専用確認する。findingの再現に必要なtestだけを実行する。
blocking findingが解決し差分が安定した後、親が読取専用 `final` を開始する。

finalは全package test/race/vet、`gofmt -l src`、`git diff --check`、`go mod tidy -diff`、
integration tagのcompile、workspace/OKF integration、Flow/GitIndependent/Assignmentの実CLI一周、
移転・配布journeyを実行する。darwin/linux/windows × amd64/arm64をbuildし、
`aidlc-dist` のarchive内容・SHA・新規配置・移転を確認する。既存CIと同じコマンドを用いる。
追加の全体lintは導入せず、既存のvet/format/CIを使う。

固定Codex CLI 0.153.4・macOSで、新しい隔離した非Git projectへ候補binaryをfresh installし、
通常のproject/hook trust後にSessionStart・UserPromptSubmit・PreToolUseを実測する。
新しい `__hook` からbootstrapが返り、help等の許可操作と未選択Intentでの一般操作の拒否を照合する。
必要なら入出力をそのまま転送する試験専用wrapperで観測し、生成された元hook設定との対応も保存する。
手作りJSONだけの呼出しを実Codex検証と呼ばない。既存fixture・実案件の状態は流用しない。
検証記録はrepository外の `ai-dd-validation/product-naming/` へ保存する。

final後に対象差分が変われば証拠はstaleとして必要なloop/reviewを経て再finalする。
PRをIssueへ紐付け、対象GitHub checks全成功後に通常のsquash mergeとIssue closeを確認する。

## 故障と復旧

rename漏れはcompile・埋込み/配置test・新hook契約testで検出し、同じ所有範囲で修復する。
配置や実機の失敗を成功扱いせず、独自設定の消去・trust迂回で回避しない。
候補検証を戻す場合は隔離試験を終了し、binaryと対応する設定を組で元に戻す。
既存利用者のKnowledge・進捗・予約を古いseedで上書きしない。
新しい保存形式や機能追加が必要になった場合は改名の範囲を超えるため別の判断として提示する。
