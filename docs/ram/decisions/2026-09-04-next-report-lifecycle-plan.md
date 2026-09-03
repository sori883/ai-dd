# 内部Next・Reportとライフサイクル一周テストの実装計画

- 日付: 2026-09-04
- 状態: Accepted（薄いライフサイクル包括承認内）
- Issue: [#83](https://github.com/sori883/ai-dd/issues/83)
- 基点: `296c5b2d05bd354fa5073566288f1f6a05ac0bcc`（PR #82）
- 許可: [薄いライフサイクルマイルストーン](2026-09-03-thin-lifecycle-milestone.md) PR7、[包括承認](2026-09-03-milestone-authorization-and-autonomous-merge.md)

## 背景・目的

Go版にはIntent開始、成果物確認、gate、承認から次Stage/全体完了までの内部操作が揃った。
本変更は呼出側が「次に何をすべきか」を読み取るNextと、作業結果を明示的に伝えるReportを接続する。
実filesystem上のStartIntentからworkflow完了までを一周するtestで、保存済み状態と監査履歴の連携を保証する。
Stageは作業工程、recordはIntent単位の保存先、facadeは既存操作をまとめる薄い内部入口である。

## Nextの契約

Nextは自身でrecordlock.Withを取得し、stateをfresh readして以下を返す。
- Running、current[-] EXECUTE: run-stage。
- Running、current[?] EXECUTE: awaiting-approval。
- Running、current[R] EXECUTE: revising。
- 整合したCompleted: workflow-complete。
zero kindは成功でない。live結果のStage metadataは所有権を分離する。複数回取得したmetadataのmap/sliceも共有しない。

[-]とCompletedは既存ResolveDirectiveを再利用し、[?]/[R]は既存gateのstate/graph検証で分類する。
ResolveDirectiveの受理契約を広げない。現在StageのSKIP、複数live、pending、phase/catalog不一致、Running/current[x]を
次Stageや完了へ読み替えない。未完Initialization、全Construction、summary/pipeline/reviewer/sensor/per-unit/CodeKB等の
未対応能力を通さない。終端には不要なgraph/artifact/audit本文の有効性を要求しない。
Nextは実行、gate開始、承認、receipt消費、state/audit/cursor/registryの変更をしない。結果は権限tokenでなく、
後のReportは改めてstate/receiptを検証する。recordlockの一時調整や読取りatimeまで完全無変更とは主張しない。

### Rootとlockの対応

audit.ValidateRecordBinding相当の小さな内部公開helperを既存read_root.goへ追加し、既存verifyReadBindings/verifyRootBindingsを共有。
context、Root非nil、保持中Guard、要求Identity一致、canonical project実体、対象space/intent record実体を検証するだけで、
audit本文、HUMAN_TURN、clone IDを読まない。lock/lease取得、directory作成、Root closeもしない。
ReadEvents入口は共通化してよいが、途中/終了時検証と厳格audit読取りは保持する。
Nextは同一lock内でbinding確認→fresh state read→binding再確認→分類。後検証失敗から結果を返さない。
gate/Approveのfresh監査読取りをbinding helperへ置換してはならない。新packageやrecordlockへの移動は不要。
非協調的外部変更の完全TOCTOU排除は保証しない。

## Reportの契約

報告種別はawaiting-approval/rejected/revised/approvedの4種類。自由文やbooleanから種類を推測しない。
それぞれOpenGate/RejectGate/ReviseGate/ApproveGateへ一度だけ委譲する。操作は下位transactionがlockを所有するので、
Reportに外側wrapper lockを置かない。approved後にさらにadvance/completeを呼ばない。
報告対象slugとCurrent stageを必須にし、両者が同じcanonical slugであることを確認する。空/未知kind、空slug、zero Current、他Stageの古い報告を拒否。
approvedはexact choice、rejectedはRequest Changes+feedbackを保持する。承認の根拠は下位のfresh ledgerで確認。
gate未開始[-]を黙って開いて承認、settled Stage再報告によるrecovery、Initialization nongated completion、
CLIのdone/complete alias parserは追加しない。
結果は操作、対象、無変更の再検証、PR6の部分commitとerrorを失わず返す。Report成功を全体完了と混同しない。
次の指示は別途Nextで取得する。

## 一周E2Eと回帰テスト

integration tag付き実filesystem testをsrc/internal/orchestrator/lifecycle_integration_test.goへ追加。
fixture準備以外は製品APIを繋ぎ、state markerを手で進めない。StartIntentで生成されたrecordを使用し、
graphは完了Initialization、ideation同phase複数、保存SKIP、inception/operationへのphase境界、最終Stageを含む。
少なくとも一つ通常成果物を要求する。実行対象に未対応Constructionを入れない。

主シナリオ:
1. StartIntentでIntent/description/stateを作り、Nextから最初の通常Stageを取得。
2. 成果物欠落のReportを拒否しstate/audit不変。fixture成果物配置後Reportでgate。
3. Next awaiting-approval、humanなしapproved拒否。
4. fixtureが既存recordlockとaudit.AppendでHUMAN_TURNを記録し、Report approvedで次Stageへ。
5. 次gateで旧receiptの再利用を拒否。新receiptでreject、Next revising。
6. Report revisedで再提出し、追加humanなし承認を拒否。新receiptで承認。
7. 同phase、SKIP、phase境界を通過し、最終Stage承認後Next workflow-complete。

確認: audit順/field、SKIP非実行、Completedは実[x]数、最終Currentは最後のsettled Stage、
InProgress/NextStage none、Revision Count保持、Next繰返しでstate/audit/registry bytes不変、
未知section/comment/改行保持、callerRoot継続利用、無関係record不変、registry status未同期。
注入scope-gridと保存suffixが異なっても保存suffix優先。
PR6全failure matrixを繰返さず、代表的部分保存をReportが保持して返すこととNextが中間[x]を回復しないことを確認。
Nextの終端は壊れたaudit/graphなしでもstate整合性とbinding確認で判定する。
binding helperはnil/canceled ctx、held/wrong/released Guard、root不一致/置換、audit欠落/破損非依存を回帰確認。
既存Directive.StageのcloneがSensors/ProducesKindsもdeep-copyするよう最小修正し所有権回帰testを追加する。

## CI接続

既存.github/workflows/ci.ymlのquality jobへ、audit/recordlock/orchestratorのintegration test実行を追加する。
既存workspace/state/memory integrationと重複させず、-tags=integration -race -count=1 -shuffle=onを使用。
既存action pin、Go matrix、権限、外部serviceを変更しない。新規tool/module追加なし。
これで一周E2EをPRのGitHub checksにも含める。クロスビルドは既存CLI matrixを維持。

## 単独writer所有範囲

- src/internal/orchestrator/next.go、report.goと対応unit/integration tests、lifecycle_integration_test.go
- directive.go/directive_test.goのdeep-copy修正、既存gate/approveのprivate helper共有は最小限
- src/internal/audit/read_root.go、read_integration_test.goのbinding helper
- .github/workflows/ci.yml
- docs/architecture.md、docs/development.md、本計画、transition-contract調査RAM、RAM索引

StartIntent/ResolveDirectiveの公開受理範囲やstate/audit形式は変えない。
固定AI-DLC 2.6.123の確認済み範囲、内部walking skeleton、DataFS/ScopesFS注入、fixture human境界を維持。
production供給元・公開CLI・registry同期・回復・新依存は追加しない。本PRで新しい意図的な恒久差分を採用しない。

## 実施順・品質gate

loopで1 behaviorずつRED/GREEN:
1. binding helperとNext freshread/nonmutation
2. 4種類、エラー/unsupported、metadata ownership（Directive回帰を含む）
3. Reportの単一委譲、必須kind/slug/Current、古い報告とchoice/feedback
4. 部分結果伝播と中間state非回復
5. StartIntentから終端の実FS E2E、reject/revise/再利用拒否/SKIP/phase/byte保持
6. 同じE2EをCIへ接続

targeted:
go test -count=1 -run '^(TestNext|TestReport|Test.*Directive)' ./src/internal/orchestrator
go test -tags=integration -count=1 -run '^TestLifecycle' ./src/internal/orchestrator
auditはbinding helperのtestのみ。gofmt適用はloop。
固定base/head独立review（verification_mode=review）後、親がread-only finalを一度実行:
go test -count=1 -shuffle=on ./...
go test -race -count=1 -shuffle=on ./...
go test -tags=integration -race -count=1 -shuffle=on ./...
go vet ./...
go vet -tags=integration ./...
gofmt -l src
go mod tidy -diff
git diff --check
darwin/linux/windows × amd64/arm64 CLI build、対象audit/orchestrator integration test binary compile。
既存CI stepと新integration stepの実行成功をGitHub checksで確認しmerge commit、main反映、Issue close。
一周テストは内部API E2Eであり配布CLIのlifecycle E2Eとは表現しない。

## 根拠・残余risk

固定aidlc-orchestrate.ts:2723（gate分類）、4561–4674（Next）、6755–6784（Report分類）、
7950–8052（gate lifecycle）、8094–8218（forward/recoveryの別経路）、
既存StartIntent integration、current-directive RAM、PR5/PR6計画を根拠とする。
本家の広いreport recoveryやproduction dispatcherは、未実装を迂回せず将来接続時の確認対象とする。
production trusted入力取得元が未接続であること、audit-firstの非対称durability、中間stateは自動修復しない制約は維持。

## 実施記録（2026-09-04）

Issue #83／PR7として、計画の順序どおり次を実装した。`audit.ValidateRecordBinding`は既存のRoot・Guard・identity検証を
audit本文の読取りから分離し、Nextがlock内でbinding→fresh state→binding再確認を行えるようにした。`Next`は4つのread-only
directive結果を保存suffixから分類し、`Directive.Stage`とstate contentの所有権を分離した。`Report`は4種類を既存gateへ一度だけ
委譲し、下位transactionのpartial result／errorを保持した。StartIntent起点のintegration testでは、成果物、fresh receipt、
reject/revise、SKIP、phase境界、終端、unknown bytes、registry未同期、無関係record、terminal後のaudit／graph／artifact欠落を一周確認した。

受入対象のGo変更は標準ライブラリのみで、公開CLI、registry同期、production dispatcher、state/audit形式の拡張は行っていない。Reportは
Current stageを省略できず、Slugとの完全一致を委譲前に検証する。CIの
quality jobにはaudit／recordlock／orchestrator integrationをrace・shuffle付きで追加した。loopでの対象確認結果は実装担当の完了報告に
記録し、全体test・race・vet・cross build・GitHub checksは親agentのfinal gateへ委譲する。
