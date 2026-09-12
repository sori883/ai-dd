# Codex・Claude Code・VS Code Copilotへの製品対応

2026-09-12。ユーザーは「ai-dlc本家に配布の仕組みがあると思うのでそれを流用しつつ、実装してください」と依頼した。本計画はこの直接依頼に基づく。対象は任意のGitHub skillだけでなくAI-DLC製品全体。CopilotはVS Code版である。

## 利用者が得る結果と維持する契約

同じGo実行ファイルから利用するAI環境に合ったskill、専門担当、hookを配置し、共通のSpace・Intent・OKF文書と工程を利用できるようにする。hookは通常のAI操作で開始条件・承認待ち・担当とworker予約を確認する接続であり、全OS書込みを禁止する仕組みではない。

Goの単一バイナリ、Git不要の運用、メインAIによる標準ツールでの子担当起動、同じ管理rootで共有するworker予約、利用者のrule.md、既存の6工程を維持する。任意skill aidlc-githubは標準配布へ追加しない。日本語チェックの別CLIも別配布のままとする。外部Go moduleを追加しない。

本家の固定AI-DLC 2.6.123にある `scripts/manifest-types.ts`、`scripts/package.ts`、`harness/{claude,codex,copilot}/manifest.ts` と該当emitを参照する。本家は共通資材を配置定義で環境ごとに投影し、表だけでは表せないhookやagentを環境別生成処理で補う。この構成を既存Go installerへ反映する。TypeScript/Bunの実行環境、旧33 Stageや旧memoryを導入する意味ではない。最新upstreamや全harnessの同等性は確認していない。

## 実行順序と確認gate

| 作業 | 完了条件 | 実装範囲 |
| --- | --- | --- |
| G0 接続前提の確認 | 固定した実機でイベント識別・拒否・子担当対応を記録し、未確認を区別する | 専用試験フォルダと調査RAM。製品の保証を推測で変更しない |
| D1 配布の共通化 | 配置定義と環境別生成をGoで分離し、Codexの全配置fileが変更前と同じbytes・pathになる | 下記の具体work unit。G0と独立して進められる |
| D2 Claude・VS Code接続 | G0で対応方法が確定し、具体的な入力変換・保存・共存・復旧契約を計画へ追記した後に実装 | installer、app、assignment、flow初期化検査とhost別資材。未確定の重要選択があれば先に確認 |
| D3 完走・配布検証 | 各環境で工程、OKF、独立review、会話承認、完了と故障復旧を確認 | 実機条件・結果を記録し、対応済みの範囲を正確に案内 |

D1とD2を分ける理由は、D2の子識別と会話承認に未検証の入力契約があり、D1は保存形式や利用者の動作を変えず独立して検証できるためである。項目数を理由とした分割ではない。D1だけを3環境対応の完了とは報告しない。

## D1の具体契約と単独writer範囲

現状は `src/internal/install/install.go` に共通資材・Codex資材の読込み、配置表、hook生成、非上書き保存が集中する。共通投影処理はsource FSとsource/destinationの対応を受け取り、環境別生成物を統合し、配置可能な相対pathだけを返す。source未存在、絶対path、親への脱出、重複destinationは保存前に拒否する。mapの列挙順にかかわらず結果は同じになる。

- 追加: `src/harness/manifest.go` と対応test。共通の配置定義と投影処理。
- 追加: `src/harness/codex/manifest.go` と対応test。既存の配置対応とhook生成を移す。
- 変更: `src/internal/install/install.go` と対応test。投影結果を既存の全件事前検査・排他的新規保存へ渡す。既設fileの非上書き、symlink拒否、partial結果を維持する。
- 必要な範囲の `src/harness/codex/assets.go`。既存source資材・公開FilesとWorkflowMarkdown、relocateのtemplate読込みは維持する。内容を移動してから作り直す必要はない。
- 文書: 本計画、調査RAM、承認RAMと索引。製品として新しいinstall actionはD1で公開しない。

単独実装担当がD1対象Go fileとtestの唯一のwriterになる。親は担当の稼働中に同worktreeを編集しない。G0の親所有試験フォルダは `/Users/const/sori883/ai-dd-validation/three-hosts-20260912/` に分離する。元 `/Users/const/sori883/ai-dd` の未commit資材は保全する。

Issue #183。`work_unit_id=three-host-distribution-d1`、`verification_mode=loop`。型・signature・slice別commandは[具体handoff](three-host-distribution-d1-work-unit.md)で定める。

1. 既存Codex全配置のcharacterization。固定したproject root・binaryに対する全file/pathの比較用証拠を変更前から作り、移動後も同一とする。既存behaviorなので初回GREENをそのまま記録し、人工REDを作らない。
2. 共通投影の正常系・不正source/path・重複destination。新関数の署名だけのscaffoldは許可し、実行可能なassertionのREDを確認して最小実装する。
3. Codex manifest/生成処理へ接続し、全bytes比較とinstall非上書き・symlink/親file競合の回帰を確認する。
4. hookの既知Markdown読取り範囲、relocate、別日本語CLI資材、標準skill15件が変わらないことを対象testで確認する。

loop対象は `go test ./src/harness/... ./src/internal/install` と、変更で影響したapp/配布の既存test名に限定する。親が全差分と対象test群を作業単位末尾に一度確認する。独立担当は `verification_mode=review` で計画適合・bytes保存・失敗時非上書きを読み取り専用で確認し、再現が必要な対象testだけを実行する。

安定後の `verification_mode=final` は親がread-onlyで一度開始する。`go test -shuffle=on ./...`、`go test -race -shuffle=on ./...`、`go vet ./...`、`gofmt -l src`、`go mod tidy -diff`、`go test -tags=integration -count=1 ./src/cmd/aidlc -run '^Test(FlowJourney|GitIndependentJourney)$'` を実施する。6OS/archのcross buildと配布native検証は対象PRで起動する既存CIに委ね、その全checks成功を確認する。

## G0の具体条件

確認済みローカル版はCodex 0.153.4、Claude Code 2.1.238、VS Code 1.135.0と組込Copilot Chat 0.63.0。最低対応版はこの存在確認だけでは決めない。

専用フォルダに、Go標準ライブラリのみでhook JSONを保存し指定の無害な試験操作だけを拒否する小さな観測用実行ファイルを置く。試験用文書・agent・hook設定だけを対象にし、利用者の既存project、ユーザー全体設定、認証情報は変更しない。Claudeはその試験設定を明示して起動し、VS Codeは専用folderの通常agent modeで操作する。権限・信頼の迂回optionを使わない。必要な通常trust操作と試験fileへの操作だけを行う。

観測は会話開始、UserPromptSubmit、成功・失敗tool、Pre拒否、同じ子の識別、終了・中断を対象とする。hostの応答を記録するG0は製品approvalの代理承認ではない。新しい外部toolの導入や契約は行わない。認証/利用枠など外部要因で実測できなければ、試した条件と停止点を記録し、fixtureテストを実機成功と呼ばない。

## D2へ進む前に確定する項目

- VS CodeがClaudeのsettingsも読む場合の二重登録回避。利用者のhookを一括無効化しない。
- hostとsessionの識別、各回答のturn対応、native tool/子IDと割当の対応。自然文から担当rootや対応IDを推測しない。Claudeの自動通知もUserPromptSubmitになることを実測したため、イベント名だけで人間の回答と認めない契約も必要。
- host間切替の停止・回収条件、保存場所とschema、既存配置の追加導入。過去の『後方互換不要』を新しい重要設計の包括承認に拡張しない。
- 初期化Sensorが確認する配置集合と証拠失効条件。共通工程の定義hashを環境切替だけで変えない。
- VS Codeの子は同じ子への追加会話を提供しないため、新しい子へ既存Unitの状況を渡す際の安全条件。

結果提出・Post・Stop・時間経過だけでworker枠を解放しない。hostのhook無効・timeoutなどの限界を維持し、できない操作拒否を約束しない。

## 配布・復旧・完了条件

D1のCLIと保存形式は変わらず、既設配置への更新操作はない。戻す必要があればコード変更のrevertと旧binaryへの交換で戻せ、利用projectの文書を削除する必要はない。D2ではhost固有の通常trust手順・追加導入・既設ユーザー設定保持・ロールバックを具体化してから配布する。

親がIssue、実装、独立review、final、PR、全CI成功を管理する。D1のPRはD1の完了のみを記録し、全体の残件をRAMで引き継ぐ。全3環境の実装と対応条件を確認するまで全体完了とはしない。
