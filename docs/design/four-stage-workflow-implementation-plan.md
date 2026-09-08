# 4ステージ・Intent進捗・Knowledge・ADRへ切り替える実装計画

状態: Accepted。実装の直接依頼と、残る2点への「1.git共有します。2.推奨でお願いします」を受領。
進捗はGit共有、担当・独立レビューは調整役AIが起動する。[回答と実装許可](../ram/decisions/2026-09-08-four-stage-runtime-approved.md)を優先する。
本文の候補は採用する方式として具体化する。Go自動起動・ローカルのみの代案は採用しない。

## 背景・目的・実装許可

現行Go版は、IntentごとにKDRを作り、一般操作後の作業記録をhookで必須にする。
ユーザーは作業日誌を不要とし、製品が進捗を管理する方式へ変更した。
利用者は「目的整理＋深掘り → 実装計画 → TDD → 統合検証」を進め、各境界でSensorの機械検査と
レビューを通過した時だけ次へ進める。Knowledgeは現行のWhat/How、ADRはアーキテクチャ判断のWhyを残す。

ユーザーの「実装を進めてほしい」「不要なAI-DLCの実装やファイルは削除しても良い」という直接依頼が
許可の根拠である。[依頼・現在地・確認事項](../ram/decisions/2026-09-08-four-stage-implementation-request.md)を参照する。
旧33 Stageの包括承認は流用しない。新たな重大な選択は実装前に確認する。
Go単一バイナリ、外部Go module追加なし、既存データの互換・移行不要、既存利用データの自動削除なしを維持する。

## 保存と識別の推奨契約

- 利用者は名前でIntentを作成・選択する。内部は固定32桁hex IDを使い、同名候補は明示選択する。
- 進捗の正本候補は `aidlc/spaces/<space>/intents/<id>/state.json`。Git共有の可否は確認中。
  現在値を一つ保存する文書であり、全操作audit・成果物snapshot・receipt台帳ではない。
- stateはschema version、不変ID、名前、保存revision、現在stage、進行状況、待機理由、再開条件、
  目的・範囲・受入条件・未確定事項・実装計画・Unit計画・成果物参照と現在有効な検査結果を持つ。
- stage値は `discovery / planning / tdd / integration` とし、日本語表示は合意した4段階にする案。
  進行状況は進行中・待機中・中断中・完了・取りやめを候補とし、開始直後はdiscoveryの進行中とする。
  Sensor待ち・レビュー待ち・修正待ちは進行状況の詳細として表示する。
- KnowledgeとADRは `knowledge/` のOKF bundleに置く。ADRは `knowledge/ADR/<concept>.md`。
  ADR要否と必要な参照を計画に記載する。不要時は理由をレビューし、空ADRの作成を要求しない。
  `intent_id`での検索と未知metadataの保持を継続する。
- session、実行中担当、Unit実行IDなどのローカル割当は `aidlc/.runtime/` に分ける。
  進捗stateの保存先をローカルのみとする回答の場合は、正本pathを含め計画を改訂する。
- 保存は期待revision/hashと排他制御、同一ディレクトリの一時ファイルからの置換を使う。
  不正ID・path逸脱・競合・破損時に黙って上書きしない。

## CLIと遷移の契約案

共通で `--space` と必要時の `--project-dir` を用いる。変更系は対象Intentと期待版を明示する。
最終flag一覧は実装前の計画確定時に揃え、hookの認識と同じparserを使う。

| 操作群 | 利用者が得る結果 |
| --- | --- |
| intent create/list/switch/show | 名前で開始・選択し、現在段階と進捗を確認 |
| intent configure | 目的・計画・未確定事項・必要成果物を期待版比較で保存 |
| intent check | 現段階のSensorを読取り専用で実行し不足を表示 |
| intent review | 対象版に結び付けた独立レビュー結果を取得・受理。起動責任は確認中 |
| intent advance | 現在対象の検査・レビュー合格を確認し、次段階へ一度だけ進行 |
| intent pause/resume/reopen | 理由付き中断、確認後再開、前提変更時の段階戻し |
| unit claim/result/integrate | 依存成立と割当競合を検査し、成果回収・統合状態を管理 |
| memory create/update/search/show/check | Knowledge・ADR・RuleをOKFとして保存・検索・参照 |

Sensorは存在・形式・参照・依存・検証対象版を確認し、内容の妥当性は独立レビューが判断する。
不合格・未実施・確認不能なら現段階を維持する。review結果だけで必要なユーザー承認を代替しない。
レビュー対象は計画・必要成果物・コード版を識別し、レビュー結果欄自身を対象hashへ含めない。
対象内容が変更されたら影響する合格を無効にする。state保存revisionの変化だけで全レビューを無効化する
自己循環を避ける。統合検証から完了にも同じgateを適用する案を含める。

## Unit・Bolt・中断

UnitはID・依存先・変更範囲・受入テスト・基準commit・成果commit・進捗を持つ。
BoltはUnitをまとめる実装反復とし、並列実行可能なUnit集合とは区別する。
推奨は調整役AIが独立したUnitを別worktreeで実行させ、CLIが割当・依存・結果を管理する方式。
Go自身が起動する回答の場合は、process管理・結果取得・停止・再開の契約を先に計画へ追加する。
共有state・Knowledge・ADRのwriterは調整役一人とし、workerは担当範囲の成果と変更提案を返す。
後続Unitは依存先の担当終了だけで開始せず、必要成果の統合を確認する。
質問待ちのUnitがあっても他のUnitを進められる場合、Intent全体は進行中にできる。
編集失敗からの再試行は同じUnit内で継続し、実状不明の中断は再開確認待ちとして現物を照合する。
state消失や期限切れだけで未着手と認定して担当を重複起動しない。

## hook・配布

一般操作後のKDR dirty解消要求を削除する。対応hookでIntent/Unit選択・必須Rule読込・実行中操作の
競合を確認し、stageの進行は必ずCLIのgateを通す。読取り・質問・修復・中断を妨げない。
通常のAI操作における漏れ防止という保証範囲を維持し、同一OS権限による全書込みを禁止する機構にはしない。
配置済みskill・Rule・reviewer定義を4ステージ方式に更新する。初期資産はGo内包、通常参照は配置済み本文。
Space名・作成と選択の分離・defaultのrule.md継承は承認済みの固定AI-DLC 2.6.123準拠を維持する。

## 変更対象と削除境界

| 対象 | 責任 |
| --- | --- |
| 新規 src/internal/flow/{document,store,transition,unit,sensor,review}.go とtest | 新しい進捗正本、排他保存、依存、検査、レビュー、遷移 |
| 新規 src/internal/filestore/ とtest | kdr依存から安全保存・lock等の必要な共通処理を分離 |
| src/internal/minimal/{command,session,hook}.go とtest | 新stateのCLI/hook接続、作業日誌必須化の廃止 |
| src/internal/cli/{cli,minimal}.go、src/cmd/aidlc/{main,minimal}.go とtest | 公開文法、help、実I/O、旧入口の整理 |
| src/internal/okfmemory/document.go とtest | OKF検索・保存・ADR配置の対応と部分失敗確認 |
| src/internal/install/、src/internal/workspace/space_create.go とtest | 新資産配置、ADRフォルダ、Space作成の整合 |
| src/core/minimal/、src/harness/codex/minimal/ | テンプレート、Rule、skill、独立reviewerの配布原稿 |
| src/cmd/aidlc/ の新workflow journey test | fresh導入から4段階・並列・中断・レビュー修正の動作証拠 |
| .github/workflows/ci.yml、docs/architecture.md、docs/development.md、docs/ram/ | 有効な検証の継続と最新の利用契約 |

旧kdr package・テンプレート・CLI・専用testは、新しい保存/進捗/ADRへ置換後に削除する。
mainの旧delivery/report/codex_stage/human_turn入口を外し、不要になった内部package・旧配布原稿・専用testを
依存閉包として確認して削除する。既存internal/okfはokfmemoryから利用されるため一括削除しない。
workspace/pathnorm/buildinfo等の共通基盤は必要性を確認して保持する。
CIの旧専用test呼出しは対象削除と一緒に整理し、新方式の同等の安全・保存・一周確認を配置する。
開発AGENTS・RAM・参照snapshot・利用データ・他のworktreeは削除対象に含めない。

## 実装順・受入条件・検証

単独実装担当が一つのIssue/PRのwork unitとして順番にtest-firstのRED→GREEN→整理を進める。
対象が安全に一つのwork unitへ収まらないことが具体的に分かった場合だけ、理由と独立境界を記録して分割する。

1. stateの識別・形式・競合・保存失敗・別Intent/Space分離。
2. ADR要否、必要成果物、依存循環・未知Unit、対象版のSensor。
3. review結果と対象版の対応、古い合格拒否、修正後の再レビュー。
4. 4段階の遷移・待機・再開・戻し・完了。飛越しと重複advance拒否。
5. Unit割当・依存・成果回収・統合・再開確認。未統合の後続起動拒否。
6. 公開CLI・hook・配置済み資産を接続し、作業日誌なしで通常作業を継続。
7. fresh一周と独立レビュー、並列2 Unitから依存Unit、中断・保存失敗・stale結果を検証。
8. 不要旧依存の除去と、有効な回帰確認の維持。

loopでは `go test -count=1 ./src/internal/flow` の追加したTest群、変更時の
`./src/internal/minimal ./src/internal/cli ./src/internal/install ./src/internal/workspace ./src/internal/okfmemory`
の該当Testを限定実行する。exactな -run はwork unit handoffで各sliceに指定する。
独立review後、安定差分に親がfinalを一度開始する。

finalは `go test ./...`、`go test -race -shuffle=on ./...`、`go vet ./...`、`gofmt -l src`、
`git diff --check`、`go mod tidy -diff`、残存と新規のintegration、6 OS/arch build、native CLI smoke、
配置したCodexで新規Intent一周・review不合格修正・再開・並列Unitのlive確認を含める。
liveは既存固定環境の実際のversionを再確認して記録し、モデルや認証設定を勝手に変更しない。
liveまたはCIの未実施・timeoutを合格に数えない。

Issueは機能開発ラベルで作成し、独立review・final・対象PRのCI成功後、通常の方式でmergeしIssue closeを確認する。
問題時は該当変更をGitで戻せる構成にし、利用先の既存データや編集済みRuleを無条件上書きしない。

## 固定本家との意図的差分

比較はローカルAI-DLC 2.6.123の確認済み範囲であり、最新upstreamとの一致は主張しない。

| 本家の確認済み挙動 | 採用する挙動 | 理由と影響 |
| --- | --- | --- |
| 33 StageとStage graph・承認・auditを中心とする | 4ステージの進捗と必須Sensor/reviewを中心とする | ユーザー指定の短い反復へ統一。旧公開経路・stateとの互換や移行は提供しない |
| 旧Intent Captureの3 Sensorは助言扱い | 新ステージ境界の必要検査を合格条件にする | 成果物不足のまま進行させない。存在だけで内容を認定せずreviewを併用 |
| 固定配布の旧memory等を使う | KnowledgeのWhat/HowとADRのWhyをOKFへ保存 | ユーザー指定の知識分担。配布先・Space操作は既承認の準拠境界を維持 |

根拠: docs/aidlc-analysis/README.md、固定core/tools/aidlc-version.ts、
docs/ram/decisions/2026-09-06-intent-capture-advisory-sensor-boundary.md、
docs/ram/decisions/2026-09-08-single-binary-space-rule-accepted.md。

## 実装前の詳細確定

Git共有と調整役AI起動を採用し、本文に残る「候補」「確認中」は回答済みRAMで確定している。
型・JSON・flag・対象hash・Unitの確認条件は [詳細契約](four-stage-workflow-contract.md) に具体化した。
