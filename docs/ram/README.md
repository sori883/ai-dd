# 開発プロジェクトRAM

- [実案件、配布・更新、利用文書を順に進める](decisions/2026-09-10-sequential-completion-request.md): ①から順に進める直接依頼。[最新構成の実案件計画](../design/latest-workflow-pilot-plan.md)でcheck help改善と6段階の実利用確認を具体化。各成果承認・通常のhook信頼確認は別に維持する。

- [担当・作業場所管理の具体計画と通常解放・復旧条件を承認](decisions/2026-09-10-native-agent-assignment-implementation-approved.md): 具体計画全体の直接実装承認。通常はメインAIの確認で解放し、初回管理開始・復元不能な記録の作り直しは人間確認。単独TDD実装・review・final・PR/checks/mergeへ進む。
- [A案の作業割当管理とメインAIによる起動を採用する](decisions/2026-09-10-managed-worker-assignment-approved.md): A案を承認。メインAIの標準ツールで起動し、CLIは管理root内の作業場所の二重割当を防ぐ。[具体計画](../design/native-agent-assignment-plan.md)を作成し、通常解放の確認者・管理記録復旧時の人間確認（Q1/Q2）を確認待ち。
- [エージェントの起動はメインAIの標準ツールで行う](decisions/2026-09-10-native-agent-launch-required.md): CLIからエージェントを起動しない指定を記録。CLIによる作業登録と区別する。記録時点ではA/B未選択で、採用は上記の後続記録へ。
- [本家の作業割当方式に合わせる変更案](decisions/2026-09-10-upstream-aligned-worker-control-options.md): A/B比較時点の提案。AはCLIの二重割当防止へ保証対象を変更し、Bは元の実worker稼働制限の検討継続。A採用は上記の後続記録へ。
- [本家の担当起動・並列作業・終了の定義](decisions/2026-09-10-upstream-agent-dispatch-and-worktree-reference.md): 固定2.6.123の担当frontmatter、コード生成の承認guard、Unitごとのworktree、Team限定claimを確認。作業完了・補助TTLと実process停止を区別し、今回のworker排他要求との境界を整理。
- [固定Codexでの担当起動制限の実測結果](decisions/2026-09-10-agent-guard-preflight-result.md): 正常応答での起動前拒否を確認。Stop後のprocess継続とhook故障時の子開始も観測。root・子ID・終了/再開の契約が未確定のため製品guardへ進まず、明示割当・停止確認等の代案を整理。
- [G0 fixtureのloop実装証拠](decisions/2026-09-10-agent-guard-preflight-evidence.md): test-first実装、独立review後の修復、固定Codexで観測したwire形式と故障挙動の詳細。最終ソースの検証証拠は対応PRへ記録する。

- [担当起動制限の推奨案とG0先行を承認](decisions/2026-09-10-stage-agent-worker-guard-approved.md): 各段階のstage-planner、Unitなし予約、1調整root集約で進める。[G0作業単位](../design/agent-guard-preflight-work-unit.md)を固定Codexで実測し、対応/root/停止・再開の不明点が残る場合は製品guardへ進まない。

- [ステージ別担当の起動制限とworkerのworktree排他の計画](decisions/2026-09-10-stage-agent-worker-guard-planning.md): 計画時点の記録（承認状態は後続RAMで置換）。[具体計画](../design/stage-agent-worker-guard-plan.md)に固定Codexの前提gate、実行予約・停止/再開・競合・TDD・配布を整理。stage-plannerの許可段階、Unitなし予約、複数調整rootの3点を整理した履歴。

- [RuleとCLIスキル分離の実装証拠](decisions/2026-09-10-project-rule-and-cli-skills-evidence.md): Issue #157、5項目のTDDと限定回帰、3ファイル移転・Rule/承認gate保持。reviewで固定Ruleのtype-only厳密化と正本変更拒否回帰を追加。finalで判明した移転E2Eの旧2ファイル期待を3ファイルへ追従し、lockのruntime初期.gitignoreだけを厳密な期待setへ追加。実機で判明した初回Space/Intent ID/help案内の移管漏れも復元。実読込は親finalへ引継ぎ。
- [プロジェクトRuleとAI-DLCスキルの責任分離](decisions/2026-09-10-project-rule-and-cli-skill-approved.md): rule.mdを利用プロジェクト専用にし、AI-DLC進行はaidlc、操作案内はaidlc-cliへ。WORKFLOW原稿廃止を直接承認。[実装計画](../design/project-rule-and-cli-skills-plan.md)へ配布・hook・Rule検査と5項目TDDを具体化。必須Rule固定pathに限る入力parser接続の必要性を追記。

- [ステージ計画担当の実装証拠](decisions/2026-09-09-stage-planner-agent-evidence.md): Issue #155、3項目のRED/GREENと配布回帰を確認。実機観測を受け、期待文書と検証証拠を分ける指示へ修正し、その3制約を配布回帰で確認。親がfresh finalを行う。
- [ステージ実行計画を提案する専用エージェント](decisions/2026-09-09-stage-planner-agent-request.md): discovery内と計画変更時にaidlc-stage-plannerへ採否・順序の立案を委譲する新要望。ユーザー承認・共有保存はメインAI。具体実装案を直接承認済み。Issue #155で実装する。

- [Intent実行計画のintegration fixture修復](decisions/2026-09-09-intent-execution-plan-integration-repair.md): Issue153、親finalで再現した初期化設定順・Unit step_id・保存retry revision期待をtest-only修復。E2E再実測は親final。

- [Intent実行計画のトップhelp残存一覧修復](decisions/2026-09-09-intent-execution-plan-review-repair-2.md): Issue153、再reviewで指摘された末尾の旧4段階一覧を6段階・計画選択と会話承認へ修正。

- [Intent実行計画の独立review修復1](decisions/2026-09-09-intent-execution-plan-review-repair-1.md): Issue153、reopen実体と再初期化順序、確定履歴head anchor、トップhelpを回帰test付きで修復。

- [Intent実行計画・会話承認のloop実装証拠](decisions/2026-09-09-intent-execution-plan-evidence.md): Issue153、6段階・step_id・選択Sensor・別承認・reopen履歴の8項目TDDと保存復旧。mandatory順序と未完了回の新ID置換、同一Draftの2承認を具体化。独立review/finalは親担当。

- [Intentごとの承認済み実行計画へ変更する依頼](decisions/2026-09-09-intent-execution-plan-request.md): 初期化→目的整理を必須とし、構成分析等を選択。途中の計画変更も都度承認。具体案への「hai」で一体実装の直接承認を確認。[実装計画](../design/intent-execution-plan-implementation.md)にschema・CLI・8項目TDD・承認/保存の検証を具体化。

- [codekb配置変更の実装証拠](decisions/2026-09-09-knowledge-codekb-evidence.md): Issue #151、配布・Sensor・修復hook・手順のTDD実測。metadata selector契約を維持し、機能Knowledgeの既定配置をcodekbへ変更。finalは親担当。

- [現行知識の保存先をcodekbへ変更](decisions/2026-09-09-knowledge-codekb-approved.md): knowledge/codekbへ配布・Sensor・手順を統一する直接実装依頼。外側のOKF検索rootと他文書folderは維持。

- [OKF作業記録の実装証拠](decisions/2026-09-09-okf-work-log-evidence.md): Issue #149、Knowledge log配置、metadataと前後hashの保存復旧、公開検索/show、配布手順のTDD実測。文書修復で現在の保存先とpending保存後の時刻固定を明記。finalは親担当。

- [Intent作業記録をOKF検索対象へ移す依頼](decisions/2026-09-09-work-log-okf-request.md): Knowledgeのlog/配下へ`<intent_id>-work-log.md`として保存し、metadata検索へ対応する。保存先補正後の直接承認を確認。

- [OKF selectorと具体的文書一覧の実装証拠](decisions/2026-09-09-stage-okf-documents-evidence.md): Issue147の7項目TDD、schema4登録、共有current/accepted版、lowercase adr配布とfixture追従。独立レビューのFIFO登録・空一覧roundtrip・help型例の修復を含む。最終検証は親PRに記録。

- [ステージ入力をOKF検索し、出力は保存先とmetadataを定義する](decisions/2026-09-09-stage-okf-selectors-approved.md): 方式への直接承認。必須入力の件数と版を検査し、出力先を明示する。Intentごとの保存先一覧と必要metadataも承認済み。[実装計画](../design/stage-okf-documents-plan.md)へ具体化。
- [ADRのフォルダ名とtypeを小文字にする](decisions/2026-09-09-lowercase-adr-request.md): `knowledge/adr/` と `type: adr` を指定。従来の大文字指定を置換し、実装反映は未実施。
- [ステージの入出力を実ファイル名で示す](decisions/2026-09-09-explicit-stage-document-paths-request.md): inputs/outputsとも具体的pathを記載する依頼。機能別Knowledge・判断別ADRの可変名を指定する方法は確認中。人間承認実装は中断を維持。

- [Stage Graph実装証拠](decisions/2026-09-09-stage-graph-evidence.md): 7項目のTDD、schema3 binding、差戻しの保存失敗・同一retry、現在手順とfresh配置のloop実測。log容量・明示TDD文書outputs・不正agentの独立レビュー修復を含む。最終検証と実機の証拠は親PRへ記録。

- [Stage Graphと段階別手順の実装依頼](decisions/2026-09-09-stage-graph-implementation-approved.md): 直接承認。outputsは文書のみ・空でも可、差戻し理由はIntent作業記録。定義変更時は元版復元または新Intent、既設自動更新は保留で承認済み。

- [差戻し理由の作業記録と文書だけのoutputs](decisions/2026-09-09-stage-rework-log-document-outputs.md): ユーザー指定を確定。差戻し理由はIntent配下Markdown、outputsは文書のみ。コード・テストのSensorは維持、実装未承認。

- [Stage Graphと段階別Markdownへの分割検討](decisions/2026-09-09-stage-graph-procedure-request.md): 読込み負担を減らす設計依頼。遷移をJSON、担当・入出力・手順を段階MDへ分ける提案。help実案件は保留、実装未承認。

- [新Sensor付き実案件の実施依頼](decisions/2026-09-09-new-sensor-pilot-request.md): 既存資材解析から4段階を完走する依頼。check helpの必須文書案内を対象候補として確認中。

- [開始・終了Sensorの実装証拠](decisions/2026-09-09-start-end-sensors-evidence.md): schema2、beginと合格版、文書/資材/実行結果、修復hookのloop証拠。Unit別成功・統合HEAD・段階別結果の親境界修復、同一検査版保存・TDD証拠役割分離・移転fixtureの独立レビュー修復、限定liveのshell wrapper証拠修復を含む。最終検証と実機の証拠は親PRへ記録。

- [開始・終了Sensorの実装を承認](decisions/2026-09-09-start-end-sensors-approved.md): ファイル別表への直接実装依頼。begin/境界check、共有版とIntent文書、schema2と旧data保持を具体化。

- [現状解析をSpace共有にし、構成図を別文書にする](decisions/2026-09-09-space-shared-analysis-and-diagram.md): Intent別の現状解析案を置換。共有文書の参照版による検査を提案し、日時・IDの一律更新を避ける。

- [4段階で使うSensorの整理案](decisions/2026-09-08-sensor-catalog-proposal.md): 共通4検査と段階別検査、既存基盤と追加候補、機械検査と独立レビューの境界。提案段階。

- [製品4担当の実装証拠](decisions/2026-09-08-product-agent-roles-evidence.md): 配置RED/GREEN、共有writer境界、旧Rule期待値の修復。固定Codexで4担当の起動・終了と全体検証を実測。

- [ステージ拡張より先に製品4担当を定義する](decisions/2026-09-08-product-four-agents-approved.md): 調査・要件整理・worker・reviewerの直接実装依頼。Sensorの2問と配布更新は後続へ保留。

- [Space入口・資材解析・OKF Sensorの実装具体案](../design/space-reverse-okf-sensor-plan.md): 現状解析/要件文書、対象資材、Sensor、担当、TDD順を具体化。合否に関わる2点は回答待ち。

- [Space入口・資材解析・OKF Sensorの実装依頼](decisions/2026-09-08-space-reverse-okf-sensor-implementation-request.md): 構成へ同意し実装を依頼。鮮度基準と変更不要の文書の扱いは回答待ち。

- [Space・既存資材解析・OKF Sensorの整理依頼](decisions/2026-09-08-space-reverse-okf-sensor-request.md): 配布更新・文書整理を保留して保持。現状の役割分担とIntent/更新日時を照合するSensor案。追加契約は未承認。

- [配置移転とUnit再割当の実装証拠](decisions/2026-09-08-clone-relocation-evidence.md): 既知参照だけの移転、停止確認後の再割当、部分保存再試行、実CLIと限定live入口。reviewで未完了中のrevision保全と日本語path処理を修復。

- [配置移転とUnit再割当の実装を承認](decisions/2026-09-08-clone-relocation-approved.md): 方式への直接承認。再試行・複数Unit・利用者設定保持・新hook信頼確認を具体化。

- [別cloneでの配置と担当割当の復旧案](decisions/2026-09-08-clone-relocation-request.md): 参照先更新と停止確認後の明示再割当を提案。方式確認待ち。

- [configure helpの実案件を四段階で完走](decisions/2026-09-08-configure-help-pilot-completed.md): 全境界のSensor・独立レビューを経て同一Intentがcompleted。実利用での拒否と修復も記録。

- [configure help実案件のTDD証拠](decisions/2026-09-08-configure-help-pilot-evidence.md): Unitなし/ありのJSON例、型・置換説明、実CLIでplanning Sensor検証。

- [configure helpの改善を実案件に採用](decisions/2026-09-08-configure-help-pilot-approved.md): 有効な設定JSON例を追加し、製品の4段階で完走する直接承認。

- [日常運用の実CLI検証結果](decisions/2026-09-08-daily-operations-evidence.md): 複数Intent、Git引継ぎ、並列競合、保存障害復旧と既存移転制約、Git競合判定のlocale依存除去。

- [実案件の完走と日常運用の確認](decisions/2026-09-08-pilot-and-daily-operations-request.md): 1・2の実施を直接依頼。実案件はconfigure helpのUnit有無JSON例に確定、日常運用は実CLI検証。

- [Knowledge CLI metadataとhelpの実装証拠](decisions/2026-09-08-knowledge-cli-frontmatter-evidence.md): C1〜C5のTDD、本文のみ入力、metadata保持、未選択help、限定live入口、confirm commit説明のreview修正。

- [CLIによるfrontmatter生成とhelp参照を実装する](decisions/2026-09-08-knowledge-cli-help-approved.md): 前案への直接承認。型・選択肢をhelpへ集約し、Skillから参照する。

- [KnowledgeのfrontmatterをCLIで生成する](decisions/2026-09-08-cli-owned-knowledge-frontmatter.md): 日時はCLIが自動設定、その他metadataは引数、本文はAIが作成する要求。詳細CLIは提案段階。

- [四段階liveの再開と証拠集計の修正](decisions/2026-09-08-flow-live-repair-evidence.md): 失敗出力、inactive手順読取、会話別再完了の実証。

- [四段階final失敗の修正証拠](decisions/2026-09-08-flow-final-repair-evidence.md): 未知Intent操作のSIGPIPE境界と同一binaryのpath表記差。

- [四段階の独立レビュー修正証拠](decisions/2026-09-08-four-stage-review-repair-evidence.md): Unit進捗・依存base・review checkout・stage別成果物・live実報告照合・日本語手順。

- [四段階製品切替の実装証拠](decisions/2026-09-08-four-stage-implementation-evidence.md): Issue #130、TDD・削除境界・新fresh/liveの親final入口。

## 目的

このディレクトリは、GoでAI-DLCを再実装する開発プロジェクトの意思決定、調査結果、
前提、未解決事項を継続的に記録する。

AI-DLCが利用プロジェクト内で管理する`aidlc/spaces/<space>/knowledge/`やDocumentKBとは
役割が異なる。ここにはAI-DLCという成果物を開発する側の知識だけを置き、AI-DLCを使って
開発される個別成果物の知識は置かない。

## 分類

- `decisions/`: 承認済みまたは再検討中の設計・運用上の意思決定
- `research/`: 参照実装、仕様、技術選択肢の調査結果

各記録には、日付、状態、背景、結論、影響、未解決事項、根拠を必要な範囲で含める。
決定を変更する場合は過去の記録を消さず、新しい記録から置換対象を参照する。

一時的な作業メモ、秘密情報、利用プロジェクト固有のAI-DLC knowledgeは保存しない。

## エージェント運用

1. 計画、設計、調査、実装を始める前に、この索引と関連記録を確認する。
2. ユーザーへ既出事項を質問する前に、関連する意思決定がないか確認する。
3. 今後の作業に影響するユーザーの回答、承認、方針変更は、原則として同じタスク内で記録する。
4. 記録を追加または更新したら、下記の索引も更新する。
5. 既存決定を変更する場合は新しい記録を作り、置換した記録への参照を残す。

この運用の必須ルールは、リポジトリルートの`AGENTS.md`に定める。

## 索引

現在のフローは **目的整理＋深掘り → 実装計画 → TDD → 統合検証**。
各ステージ間にSensorとレビューを設け、必要な検査とレビューに合格してからstateを次へ進める。
**Knowledgeは現行で何を・どう実現しているか、ADRはアーキテクチャ設計のなぜ**を記録する。
整理・深掘りの成果もこの分担で反映し、ADRはSpaceの `knowledge/ADR/` 配下にOKF文書として置く。
Intent・Unitの進捗はstateで管理する。全作業の日誌は要求せず、差戻し理由だけは`aidlc/spaces/<space>/knowledge/log/<intent_id>-work-log.md`へOKF文書として保存する。
過去の記録にある旧称KDRと実装識別子は履歴として保持し、名称とstateの最新方針は先頭の記録を参照する。

| 種別 | 記録 | 状態 |
| --- | --- | --- |
| 不具合・方針 | [native子hookと親会話stateの分離](decisions/2026-09-10-native-child-hook-separation.md) | Issue #161の実機で子の通常作業を誤拒否。子通知の親state分離・担当/開始条件・管理変更拒否を実装、有限rendezvousを追加。fresh実機は未実施 |
| 実装 | [native担当・作業場所管理の実装loop](decisions/2026-09-10-native-agent-assignment-implementation.md) | Issue #161、単独writer TDD・横断予約・native task対応・明示解放。実機結果と子hook修正は後続RAMを参照 |
| 意思決定 | [Git共有stateと調整役AIによる起動を採用する](decisions/2026-09-08-four-stage-runtime-approved.md) | Accepted。提示した4ステージ実装計画の残る2点を確定し実装へ進む |
| 意思決定 | [4ステージ方式の実装着手と不要製品コードの削除を依頼された](decisions/2026-09-08-four-stage-implementation-request.md) | 実装・不要製品コード削除の直接依頼。進捗共有とAI起動責任を確認中 |
| 計画 | [4ステージ方式への実装計画](../design/four-stage-workflow-implementation-plan.md) | 対象・保存/CLI案・削除境界・受入/検証を具体化。2点への回答後に契約を確定 |
| 意思決定 | [ステージ間にSensorとレビューを設ける](decisions/2026-09-08-stage-sensor-review-gates.md) | 必須のステージ間検査・レビューを確定。具体的な検査・担当・完了条件は計画で定める |
| 意思決定 | [Knowledgeは現行のWhat・How、ADRはWhyを担う](decisions/2026-09-08-knowledge-what-how-adr-why.md) | 方針確定。ADRはSpaceのknowledge/ADR/へ配置。製品反映は実装計画で扱う |
| 意思決定 | [目的整理・深掘りの内容をKnowledgeへ保存する](decisions/2026-09-08-discovery-content-in-knowledge.md) | 方針確定。SpaceのOKFへ整理内容・回答・調査結果・未確定事項を保存 |
| 意思決定 | [4段階のフローとADR・進捗stateの責任を確定する](decisions/2026-09-08-four-step-flow-adr-and-progress-state.md) | 方針確定。各工程のADR作業記録必須化を置換。具体的なstate・Sensor契約は未確定 |
| 意思決定 | [Intent全体のstate管理を要件に加える](decisions/2026-09-08-intent-state-management-required.md) | Intentの状態を製品が管理・制御する要求を確定。具体的な状態・遷移・保存形式は検討案 |
| 意思決定 | [ADRへの名称統一とUnit実行stateの具体案](decisions/2026-09-08-adr-name-and-unit-runtime-state.md) | ADRへの名称統一は確定。割当・実行状況・再開確認のstateは検討案 |
| 意思決定 | [KDRを正本にしたUnit実行管理とSensorの案](decisions/2026-09-08-kdr-unit-state-sensor-proposal.md) | ADR=IntentごとのKDRを確定。最小実行state・成果物Sensorは検討案 |
| 意思決定 | [作業全体でOKFを参照し、各段階の判断と結果を記録する](decisions/2026-09-08-okf-reference-and-record-every-step.md) | 要件を記録（各段階の参照・行動・判断・結果。ADRは既存KDRを指すと上記で確定） |
| 意思決定 | [やること・未確定事項の深掘りを先に行い、実装を分割して並列化する](decisions/2026-09-08-discovery-before-parallel-implementation.md) | 方向性を記録（Bolt/Unitを区別。具体契約・次期実装は未承認） |
| 意思決定 | [編集失敗後も同じIntentで作業を続ける](decisions/2026-09-08-edit-failure-remains-in-progress.md) | Accepted（AIが停止を確認して復旧・再試行。未記録を保持し同じKDRへ保存） |
| 調査 | [編集失敗時の終了通知欠落と明示復旧の確認](research/2026-09-08-minimal-live-failure-recovery-gate.md) | Resolved（上記の継続方針を承認。失敗の実機証拠は保持） |
| 意思決定 | [M1最小Intent実装の検証証拠](decisions/2026-09-08-m1-implementation-evidence.md) | Review修正のLoop完了（索引破損判定・実機証拠の補強。再review・finalは後続gate） |
| 意思決定 | [初回の操作案内をSessionStartから渡す](decisions/2026-09-08-minimal-bootstrap-context.md) | Accepted implementation detail（配置済み最小skillだけ、必須Rule本文はCLI読込を維持） |
| 意思決定 | [CLIとhookの一時保存先をworktreeへ揃える](decisions/2026-09-08-minimal-runtime-workspace-path.md) | Accepted implementation detail（aidlc/.runtime、Git対象外。正本・権限は維持） |
| 調査 | [固定Codexの最小hook実機確認](research/2026-09-08-minimal-hook-live-preflight.md) | Preflight passed（15 hook入力・8 transport呼出し、Pre拒否・非同期Post・Stop再入を実証） |
| 意思決定 | [M1実装計画と直接承認](decisions/2026-09-08-m1-implementation-plan.md) | Accepted / 実装着手（OKF・KDR・hook・名前操作・ID検索。固定hook実証を先頭gateとする） |
| 意思決定 | [OKF検索でintent_idを指定できるようにする](decisions/2026-09-08-okf-intent-id-search.md) | Accepted（Space内のfrontmatter完全一致検索。標準metadata利用も採用） |
| 意思決定 | [KDRでもOKFの標準metadataを利用する](decisions/2026-09-08-kdr-okf-metadata-clarification.md) | Clarified（description・tags・generated・status等の用途を補足。識別3項目に限定しない） |
| 意思決定 | [Intentを名前で選び、KDRのfrontmatterに識別情報を保存する](decisions/2026-09-08-intent-name-frontmatter-accepted.md) | Accepted（名前で作成・選択、intent_idとtitleを保存。会話との対応は内部管理） |
| 意思決定 | [M1着手前の確認とIntent操作の未決事項](decisions/2026-09-08-m1-readiness-intent-operation-question.md) | Resolved（質問履歴。上記の名前操作・frontmatterの合意で解消） |
| 意思決定 | [OKF・初期資産の内包、OKF Space作成、rule.md継承を採用する](decisions/2026-09-08-single-binary-space-rule-accepted.md) | Accepted（3点ともA。組織Ruleはrules/rule.mdを作成時コピー） |
| 意思決定 | [Goシングルバイナリ継続とAI-DLC準拠範囲の確認](decisions/2026-09-08-single-binary-and-conformance-questions.md) | Resolved（質問履歴。回答は上記の採用記録） |
| 意思決定 | [Space作成もAI-DLCに準拠する](decisions/2026-09-07-minimal-space-creation-conformance.md) | Accepted（作成契約を固定本家基準とし、OKF初期化との接続をM1計画で具体化） |
| 意思決定 | [Space内の配置案を採用し、配布の仕組みはAI-DLCに準拠する](decisions/2026-09-07-minimal-layout-distribution-conformance.md) | Accepted（下位分類を含む配置案を採用。本家の生成・配置方式を配布設計へ反映） |
| 意思決定 | [SpaceのknowledgeへKnowledge・KDR・RuleをOKFとして集約する](decisions/2026-09-07-space-knowledge-okf-unification.md) | Accepted（配置・全文書のOKF対応を直接指定。M1実装は未承認） |
| 意思決定 | [OKF Go実装のローカル参照先](decisions/2026-09-07-okf-local-implementation-reference.md) | Accepted（`docs/実装_okf-agent-memory/`を実装参考に指定。元commitは未確認） |
| 意思決定 | [M0最小契約を具体化した案と、未承認のM1境界](decisions/2026-09-07-minimal-product-contract-proposal.md) | Proposed（Intent/KDR/CLI/hook/OKFとM1対象・検証を具体化。契約採用・M1実装は未承認） |
| 意思決定 | [新方式では既存Intent・stateの移行を要件にしない](decisions/2026-09-07-minimal-product-no-legacy-migration.md) | Accepted（既存は無視してよいとの回答。移行・互換性・新旧二重運用を要求しない） |
| 意思決定 | [現在のGo実装からOKF・Intent KDRへ移るマイルストーン案](decisions/2026-09-07-minimal-product-milestones.md) | Proposed（mainとPR履歴を棚卸し。M0契約→M1一周実証→M2運用→M3配布・縮小。既存移行不要の回答を反映） |
| 意思決定 | [Intent単位のKDR記録とhookの保証範囲](decisions/2026-09-07-intent-kdr-hook-boundary.md) | Accepted（Intentごとの作成・記録を必須化。通常のAI操作の記録漏れ防止。実装計画全体は未承認） |
| 意思決定 | [OKF・KDR・hookへ製品構成を絞る方針](decisions/2026-09-07-okf-kdr-minimal-product-direction.md) | 方向性を記録、具体設計はProposed（hookの保証範囲・KDR保存単位は後続決定で確定） |
| 意思決定 | [合意・成果物・検証・再開を中心にするworkflowの設計検討](decisions/2026-09-07-artifact-centered-workflow-proposal.md) | Proposed（ユーザーの設計依頼。Stageを案内へ変更する案、共有成果物・再開・根拠・監査・移行を検討。実装は未承認） |
| 意思決定 | [初期実装の境界](decisions/2026-08-29-initial-implementation-boundaries.md) | Accepted |
| 意思決定 | [プロジェクトRAMの記録運用](decisions/2026-08-29-project-ram-policy.md) | Accepted |
| 意思決定 | [Project root解決の初期契約](decisions/2026-08-30-project-root-resolution.md) | Accepted |
| 意思決定 | [Local配布E2E sandboxの運用](decisions/2026-08-30-local-distribution-e2e-sandbox.md) | Accepted |
| 意思決定 | [内部workspace機能を先行し、statusを後段で実装する](decisions/2026-08-31-internal-workspace-before-status.md) | Accepted（実装順序） |
| 意思決定 | [共通space読み取りの初期契約](decisions/2026-08-31-space-reading-contract.md) | Accepted |
| 意思決定 | [Intent読み取りの実装計画](decisions/2026-08-31-intent-reading-plan.md) | Accepted |
| 意思決定 | [Workspace読み取り接続の実装計画](decisions/2026-08-31-workspace-reading-composition-plan.md) | Accepted |
| 意思決定 | [Space作成をCLIから使えるようにする実装計画](decisions/2026-08-31-space-creation-plan.md) | Accepted（Issue #19、strict flag値・SIGPIPE修正・最終配布E2Eまで記録） |
| 意思決定 | [Space一覧をCLIへ接続する実装計画](decisions/2026-08-31-space-list-plan.md) | Accepted（Issue #21、引数境界の具体化、TDD・独立レビュー・53起動の配布E2Eを記録） |
| 意思決定 | [Space切替を共有カーソルへ接続する実装計画](decisions/2026-09-01-space-switch-plan.md) | Accepted（Issue #23、17項目TDD・独立レビュー・76起動の配布E2Eを記録） |
| 意思決定 | [Intent一覧をCLIへ接続する実装計画](decisions/2026-09-01-intent-list-plan.md) | Accepted（Issue #25、16項目TDD・P1修正後の独立レビュー・45起動の配布E2Eを記録） |
| 意思決定 | [Intent切替を共有カーソルへ接続する実装計画](decisions/2026-09-01-intent-switch-plan.md) | Accepted（Issue #29、13項目TDD・P2/P3修正後の独立review・32起動の配布E2Eを記録） |
| 意思決定 | [Intent作成の内部coreとworkspace lockの実装計画](decisions/2026-09-01-intent-create-core-plan.md) | Accepted（Issue #31、13項目TDD＋Go 1.27回帰修正・review・6構成cross compile、PR #32） |
| 意思決定 | [読み取り専用ワークスペース分析の実装計画](decisions/2026-09-02-workspace-detection-plan.md) | Accepted（Issue #33、7項目TDD・P1修正後の独立review・6構成cross compile） |
| 意思決定 | [Stage graph・scope routing内部APIの実装計画](decisions/2026-09-02-stage-routing-plan.md) | Accepted（Issue #35、TDD・P1/P2修正後review・6構成cross compile） |
| 意思決定 | [Scope metadata read-only APIの実装計画](decisions/2026-09-02-scope-metadata-plan.md) | Accepted（Issue #37、7項目RED/GREEN＋block-first・ECMAScript trim・raw改行とinner backtrackingを含むblock regex parity修正・Runner ownership guard・最終review指摘なし・6構成cross compile） |
| 意思決定 | [初期 aidlc-state.md builderの実装計画](decisions/2026-09-02-initial-state-builder-plan.md) | Accepted（Issue #43、5項目TDD・P1修正後の独立review完了） |
| 意思決定 | [初期state永続化writerの実装計画](decisions/2026-09-02-initial-state-writer-plan.md) | Accepted（Issue #45、独立review・final完了） |
| 意思決定 | [4層Memory source readerの実装計画](decisions/2026-09-02-memory-source-reader-plan.md) | Accepted（Issue #47、独立review・final完了、Go 1.26.8で全検証） |
| 意思決定 | [Memory bundle filterの実装計画](decisions/2026-09-02-memory-bundle-filter-plan.md) | Accepted（Issue #49、独立review完了、final検証結果はPRへ記録） |
| 意思決定 | [検証頻度をloop・review・finalへ分離する](decisions/2026-09-02-validation-cadence.md) | Accepted（Issue #39、修正中はtargeted、差分安定後に全検証を1回） |
| 意思決定 | [Go TDDの依頼をREDとGREENへ分離する](decisions/2026-09-04-tdd-phase-handoff-plan.md) | Superseded（履歴は保持。1項目ごとの親子往復は下記work unit方式へ置換） |
| 意思決定 | [Go TDDを作業単位の連続実装へ変更する](decisions/2026-09-05-tdd-work-unit-handoff.md) | Implemented（Issue #110、各項目のtest-firstを維持し、全項目完了後に1回返却。隔離forward test成功） |
| 意思決定 | [go_tdd_implementerをLuna / maxで運用する](decisions/2026-09-02-go-tdd-implementer-luna-max.md) | Superseded（履歴は保持。下記Astra / low運用へ置換） |
| 意思決定 | [go_tdd_implementerをGPT-6 Astra / lowで運用する](decisions/2026-09-06-go-tdd-implementer-astra-low.md) | Accepted（Issue #121、新規起動する実装担当をAstra / lowへ固定） |
| 意思決定 | [サブエージェントhandoffのコンテキスト予算](decisions/2026-09-03-subagent-context-budget.md) | Accepted（Issue #59、全文継承を例外化し、調査担当はTerra / mediumを維持） |
| 意思決定 | [本家AI-DLCとの差分を自発的に提示する](decisions/2026-08-31-upstream-difference-reporting.md) | Superseded（下記の意図的な差分に限定する方針へ置換） |
| 意思決定 | [本家との差分提示を意図的な仕様・挙動の変更に限定する](decisions/2026-08-31-intentional-upstream-difference-reporting.md) | Accepted |
| 意思決定 | [GitHub Issue・PRを日本語の実装記録として運用する](decisions/2026-09-01-japanese-github-pr-workflow.md) | Partially Superseded（日本語・履歴確認は維持、自動マージ禁止は下記決定で置換） |
| 意思決定 | [ロードマップ単位の包括承認と自律マージを採用する](decisions/2026-09-03-milestone-authorization-and-autonomous-merge.md) | Accepted（Issue #67、現在のAI-DLC Go実装ロードマップを最初の包括承認枠とし、品質gate後に自律マージ） |
| 意思決定 | [GitHub Issueを主要な成果で分類する](decisions/2026-09-01-github-issue-classification-labels.md) | Accepted（`機能開発` / `ユーザーリクエスト`、全14 Issueへ適用） |
| 意思決定 | [計画・Issue・PRを自己完結した分かりやすい文章にする](decisions/2026-09-02-self-contained-development-artifacts.md) | Accepted（Issue #51、今後生成する成果物へ適用） |
| 意思決定 | [OKF v0.2参照基盤と初期統合境界](decisions/2026-09-03-okf-reference-boundaries.md) | Partially Superseded（Issue #53、Stage実行中固定はin-flight recompose方針で置換、その他のOKF境界は維持） |
| 意思決定 | [Space固有knowledgeをOKF metadataで段階的に検索する](decisions/2026-09-07-okf-metadata-knowledge-search-plan.md) | Accepted（Bundle root、検索・順位・上限、YAML module、legacy cutover、本家との差分を2026-09-07に直接承認） |
| 意思決定 | [AI-DLC Go実装ロードマップ（概要）](decisions/2026-09-03-aidlc-implementation-roadmap.md) | Partially Superseded（全体順序は維持、Stage実行中固定とPRごとの承認待ちは後続決定で置換） |
| 意思決定 | [Ideation Stage共通の質問・summary receipt基盤を作る](decisions/2026-09-07-stage-generic-summary-receipt-plan.md) | Accepted（Issue #126、ロードマップ第4段階の最初の共通基盤。固定2.6.123のreceipt schemaを維持し、非intent Stageのproduction gateは開かない） |
| 意思決定 | [Stage catalog metadataの実装計画](decisions/2026-09-03-stage-catalog-metadata-plan.md) | Accepted（Issue #55、TDD・loop検証・独立review・final gateを記録） |
| 意思決定 | [Intent開始時Stage Plan builderの実装計画](decisions/2026-09-03-stage-plan-builder-plan.md) | Accepted（Issue #57、ユーザー明示承認済み） |
| 意思決定 | [StartIntent内部接続の実装計画](decisions/2026-09-03-start-intent-plan.md) | Accepted（Issue #61、ユーザー明示承認済み） |
| 意思決定 | [in-flight recompose方針](decisions/2026-09-03-inflight-recompose-policy.md) | Accepted（旧ロードマップ／Stage Planの実行中固定方針を置換対象として参照） |
| 意思決定 | [保存済み aidlc-state.md readerの実装計画](decisions/2026-09-03-state-reader-plan.md) | Accepted（Issue #63、ユーザー明示承認済み） |
| 意思決定 | [Current directive resolverの実装計画](decisions/2026-09-03-current-directive-resolver-plan.md) | Accepted（Issue #65、ユーザー明示承認済み） |
| 意思決定 | [Stage completion artifact presenceの実装計画](decisions/2026-09-03-stage-artifact-presence-plan.md) | Accepted（Issue #69、ロードマップ包括承認内） |
| 意思決定 | [薄いライフサイクルを内部APIで完走するマイルストーン](decisions/2026-09-03-thin-lifecycle-milestone.md) | Accepted（ユーザーが残り7 PRの内部walking skeletonを明示承認） |
| 意思決定 | [Stage完了可否のread-only判定計画](decisions/2026-09-03-stage-completion-decision-plan.md) | Accepted（Issue #71、薄いライフサイクルマイルストーン内） |
| 意思決定 | [byte-preserving state transition patcherの実装計画](decisions/2026-09-03-state-transition-patcher-plan.md) | Accepted（薄いライフサイクルマイルストーン内） |
| 意思決定 | [既存state atomic update writerの実装計画](decisions/2026-09-03-state-update-writer-plan.md) | Accepted（薄いライフサイクルマイルストーン内） |
| 意思決定 | [最小audit ledgerとrecord lockの実装計画](decisions/2026-09-03-audit-record-lock-plan.md) | Accepted（薄いライフサイクルマイルストーン内） |
| 意思決定 | [承認ゲート遷移と人間応答監査記録の接続計画](decisions/2026-09-04-approval-gate-receipt-plan.md) | Accepted（Issue #79、薄いライフサイクルPR5、reader所有の承認根拠・ECMAScript trim回帰修正を記録） |
| 意思決定 | [承認から次Stage・workflow完了までの接続計画](decisions/2026-09-04-approve-advance-plan.md) | Accepted（Issue #81、薄いライフサイクルPR6、二段階audit-first保存・実装記録） |
| 意思決定 | [内部Next・Reportとライフサイクル一周テストの計画](decisions/2026-09-04-next-report-lifecycle-plan.md) | Accepted（Issue #83、薄いライフサイクルPR7、内部入口・一周E2E・CI・Report対象拘束の回帰修正） |
| 意思決定 | [ルール・知識・工程定義を本文入りの検討用資産として配置する](decisions/2026-09-04-aidlc-content-baseline.md) | Partially Superseded（Issue #85、配置先だけを下記core構成へ置換。原文保持・OKF後段の境界は維持） |
| 意思決定 | [原稿の配置を本家core構成へ揃える](decisions/2026-09-04-aidlc-core-layout.md) | Accepted（Issue #87、本家準拠を基本とし、140件をsrc/coreへ無変更移動する直接承認） |
| 意思決定 | [配置Markdownからルールと知識を供給する](decisions/2026-09-04-file-based-knowledge-delivery.md) | Accepted（ファイル供給・埋込み禁止・次回読込みへの編集反映、OKF化しない3段階の直接承認） |
| 意思決定 | [ルール・知識のAI供給を個別承認なしで完了まで進める](decisions/2026-09-05-context-delivery-autonomous-authorization.md) | Accepted（配信・Codex実読込・一連の検証を含む全範囲。品質gate後の自律merge、製品の人間承認は維持） |
| 意思決定 | [必須ルール本文を配信用のまとまりへ分割する](decisions/2026-09-05-steering-chunks-plan.md) | Accepted（純粋なChunkRules、見出し優先・JSON容量・日本語保持・path超過境界、知識供給の包括承認内） |
| 意思決定 | [必須ルールbundleのdigestとload-steering JSONを組み立てる](decisions/2026-09-05-steering-load-wire-plan.md) | Accepted（ordered digest、JSON.stringify互換wire、28 KiB境界、知識供給の包括承認内） |
| 意思決定 | [状態Markdownから工程のDepth設定を厳密に読み取る](decisions/2026-09-05-state-depth-reader-plan.md) | Accepted（Issue #101、保存済みstateのScope Configurationから一意なDepthを取得、知識供給の包括承認内） |
| 意思決定 | [工程の成果物名をIntent record相対pathへ解決する](decisions/2026-09-05-artifact-path-resolver-plan.md) | Accepted（Issue #103、通常Stageのconsume／produce path、知識供給の包括承認内） |
| 意思決定 | [配信chunk継続tokenとfreshness検証を純粋APIとして実装する](decisions/2026-09-05-steering-continuation-token-plan.md) | Accepted（HMAC token codec・freshness・part進行、key／cursor I/Oは後続、知識供給の包括承認内） |
| 意思決定 | [配信継続token用のprivate key lifecycleを実装する](decisions/2026-09-05-steering-token-key-lifecycle-plan.md) | Accepted（record／session key、32-byte random・0600・fresh read・並行初回生成、cursorは後続、知識供給の包括承認内） |
| 意思決定 | [配置済み情報からcanonical run-stageをread-only構成する](decisions/2026-09-05-run-stage-composition-plan.md) | Implemented（Issue #109、fresh配置読込・canonical wire・route/state/rule freshness、知識供給の包括承認内） |
| 意思決定 | [Codex向け配信transactionと公開next／continueを接続する](decisions/2026-09-05-delivery-publication-plan.md) | Implemented（Issue #113、active-directive v2、same-token exactly-once、公開CLIと配布journey、temporaryのidentity/content snapshot検証。破損markerは`next`がfresh recovery、`continue`はtyped error） |
| 意思決定 | [Codex receiverで配信本文を実読込する](decisions/2026-09-05-codex-receiver-read-plan.md) | Partially Superseded（旧skillのlive実読込は履歴として保持。shell直接読込とtest設計は下記契約で置換） |
| 意思決定 | [Codexのcontext読込をGoの安全境界へ移す](decisions/2026-09-05-codex-safe-context-read-contract.md) | Implemented（PR #116、Issue #115 close。`codex-cli 0.153.4`／`gpt-5.6-luna`でlive E2E成功。publication generation、plan-wide cross-file commitment、bounded streaming、canonical token、read-only snapshotを含む） |
| 意思決定 | [`intent-capture`を最初の通常Stageとして縦に通す](decisions/2026-09-06-intent-capture-vertical-milestone.md) | Partially Superseded（Issue #118から開始。公開`report`と2 PR構成は維持。`HUMAN_TURN`認証保証とsensor必須証拠は下記の決定で置換） |
| 意思決定 | [`HUMAN_TURN`を固定本家相当の運用証拠として扱う](decisions/2026-09-06-human-turn-operational-evidence.md) | Accepted（`UserPromptSubmit`を正規emit pathとするが、同じ利用者権限に対する認証済み著者証明とは扱わない） |
| 意思決定 | [`intent-capture`のsensorを固定本家どおりadvisoryとして扱う](decisions/2026-09-06-intent-capture-advisory-sensor-boundary.md) | Accepted（3 sensorはすべて起動し、得られた結果は表示・記録するが、terminal receiptや結果をgate authorityにしない） |
| 意思決定 | [`intent-capture`の残存境界を固定本家へ揃える](decisions/2026-09-06-intent-capture-upstream-conformance-corrections.md) | Accepted（learningsはStageごと1回、実memory／manifest保存、semantic summary digest、全artifact review bindingとrevision challengeへ訂正） |
| 意思決定 | [配置ファイルから必須ルール本文を毎回読み込む](decisions/2026-09-04-required-rule-delivery-plan.md) | Accepted（Issue #89、知識供給の第一slice、必須本文reader） |
| 意思決定 | [工程の必須ルール参照を配置ファイルへ解決する](decisions/2026-09-04-stage-rule-path-resolution-plan.md) | Accepted（Issue #91、graph参照の保持・active Space/配置先解決・毎回読込みの内部接続） |
| 意思決定 | [利用先の配置Markdownと知識の固定順を採用する](decisions/2026-09-04-installed-context-source-and-order.md) | Accepted（利用先配置からの読込み・UTF-16固定順と上限への影響を直接承認） |
| 意思決定 | [工程・担当AIに応じて配置知識ファイルを選択する](decisions/2026-09-04-knowledge-roster-plan.md) | Accepted（配置knowledge roster・Minimal/plugin選択・固定順・容量制限） |
| 意思決定 | [知識一覧実装の手順不備と回復方針](decisions/2026-09-04-knowledge-roster-tdd-recovery.md) | Superseded（履歴は保持。採用確認待ちは下記の直接承認で解消） |
| 意思決定 | [知識一覧の既存実装を補強して採用する](decisions/2026-09-05-knowledge-roster-recovery-approved.md) | Accepted（Issue #93、直接承認に基づく段階別修正・補足test・再review・検証。警告pathの二重escape補修を含む） |
| 意思決定 | [知識供給の容量・日本語・開発OSと配布OSの検証方針](decisions/2026-09-04-knowledge-validation-scope.md) | Accepted（容量は機械的処理、日本語利用、Mac/Linux開発testと3OS配布対応を分離。TDD原因も記録） |
| 調査 | [既存AI-DLCの配布形式](research/2026-08-29-existing-distribution-format.md) | Current for local v2.6.123 snapshot |
| 調査 | [共通space読み取りの参照契約](research/2026-08-31-space-reading-contracts.md) | Current for local v2.6.123 snapshot |
| 調査 | [Space作成の参照契約](research/2026-08-31-space-creation-contracts.md) | Current for local v2.6.123 snapshot（U+0130小文字化の追加調査を含む） |
| 調査 | [Space一覧CLIの参照契約](research/2026-08-31-space-list-contracts.md) | Current for local v2.6.123 snapshot（public parserとsession選択の境界） |
| 調査 | [Space切替の参照契約と保存API](research/2026-09-01-space-switch-contracts.md) | Current for local v2.6.123 snapshot（共有cursor・後続処理の保存先、Go1.26.4保存境界） |
| 調査 | [Intent一覧CLIの参照契約](research/2026-09-01-intent-list-contracts.md) | Current for local v2.6.123 snapshot（registry・directory相関、表示、public parser境界） |
| 調査 | [Intent切替CLIの参照契約](research/2026-09-01-intent-switch-contracts.md) | Current for local v2.6.123 snapshot（対象解決・cursor・session副作用・Go保存境界） |
| 調査 | [Intent作成coreの参照契約](research/2026-09-01-intent-create-contracts.md) | Current for local v2.6.123 snapshot（UUIDv7・registry・Bun Windows ICU / Go Unicode overlay・部分失敗） |
| 調査 | [読み取り専用ワークスペース分析の参照契約](research/2026-09-02-workspace-detection-contracts.md) | Current for local v2.6.123 snapshot（root signal・nested depth 3・言語閾値・framework/build・submodule） |
| 調査 | [Stage graph・scope routingの参照契約](research/2026-09-02-stage-routing-contracts.md) | Current for local v2.6.123 snapshot（runtime転置・scope metadata境界・fail-closed差分） |
| 調査 | [Scope metadata readerの参照契約](research/2026-09-02-scope-metadata-contracts.md) | Current for local v2.6.123 snapshot（frontmatter block-first・ECMAScript whitespace・raw改行とinner backtrackingを含むblock regex境界・validation・3件の意図的差分） |
| 調査 | [初期 state builderの参照契約](research/2026-09-02-initial-state-builder-contracts.md) | Current for local v2.6.123 snapshot（canonical state・JSON.stringify互換sidecar・routing・Greenfield補正） |
| 調査 | [4層Memory source readerの参照契約](research/2026-09-02-memory-source-reader-contracts.md) | Current for local v2.6.123 snapshot（4層fixed path・fresh read・UTF-8 fail-closed・Root境界） |
| 調査 | [Memory bundle filterの参照契約](research/2026-09-02-memory-bundle-filter-contracts.md) | Current for local v2.6.123 snapshot（substantive判定・ECMAScript trim・comment除去・preamble） |
| 調査 | [Stage catalog metadataの参照契約](research/2026-09-03-stage-catalog-metadata-contracts.md) | Current for local v2.6.123 snapshot（成果物・consume・依存edge・runtime/compiler境界） |
| 調査 | [Intent開始時Stage Planの参照契約](research/2026-09-03-stage-plan-contracts.md) | Current for local v2.6.123 snapshot（Plan解決・依存advisory・runtime再読込境界） |
| 調査 | [StartIntentとin-flight recomposeの参照契約](research/2026-09-03-start-intent-recompose-contracts.md) | Current for local v2.6.123 snapshot（initializer seam・partial Intent・state suffix source・future recompose境界） |
| 調査 | [保存済み state readerの参照契約](research/2026-09-03-state-reader-contracts.md) | Current for local v2.6.123 snapshot（canonical state・strict parse・Root境界・意図的差分） |
| 調査 | [Current directive resolverの参照契約](research/2026-09-03-current-directive-contracts.md) | Current for local v2.6.123 snapshot（Branch 10・terminal 2形・suffix authority・意図的差分） |
| 調査 | [Stage completion artifact presenceの参照契約](research/2026-09-03-stage-artifact-presence-contracts.md) | Current for local v2.6.123 snapshot（通常Stage any-of存在、filename例外、段階的境界） |
| 調査 | [薄いライフサイクルのreport・approval・state遷移契約](research/2026-09-03-thin-lifecycle-transition-contracts.md) | Current for local v2.6.123 snapshot（guard順、marker、audit-first、state advance、PR6接続確認） |
| 調査 | [知識配信を工程へ接続する前提調査](research/2026-09-05-context-delivery-stage-prerequisites.md) | Current for local v2.6.123 snapshot（Depth、artifact path、固定catalogと現在のgate能力境界） |
| 調査 | [固定AI-DLC 2.6.123の33 StageとGo production能力の対応調査](research/2026-09-07-stage-phase-production-capability-matrix.md) | Current for repository-pinned v2.6.123（5 phase・33 Stageのdelivery/completion/artifact/summary/review/sensor/pipeline/per-unit/CodeKBと不足） |
| 調査 | [Intent候補列挙・現在intent解決の参照契約](research/2026-08-31-intent-reading-contracts.md) | Current for local v2.6.123 snapshot |
