# Intentの実行計画・会話承認・進捗を一体化する実装計画

状態: 実装許可済み。対応Issue: [#153](https://github.com/sori883/ai-dd/issues/153)。対象mainは29f4344（PR152）。[承認済み設計](intent-execution-plan-proposal.md)に対するユーザーの「hai」を直接承認の根拠とする。以下は同じ範囲の実装詳細であり、旧33 Stageロードマップの包括承認は用いない。

## 利用者が得る結果

現在はdiscovery→planning→tdd→integrationの固定順である。変更後はSpace初期化、目的整理・深掘りを必須とし、構成分析、実装計画、TDD、統合検証の採否と順序をIntentごとに選べる。選んだ計画と変更案は利用者が会話で承認し、AIがCLIに出典を記録する。各実行の終わりにはSensor（機械検査）、独立レビュー、人間の成果承認を別々に確認する。計画だけの承認で未実行作業を完了にはしない。

## 保存と実行の契約

- カタログschema2は6ステージと必須prefixを持ち、固定advanceとcompletion_stageを持たない。state schema5は計画、進捗、履歴の先頭参照を一体保存する。古い形式は明示エラーとし、移行・削除・既設ファイルの上書きはしない。
- ExecutionPlanはrevision、Approved、Draft、NextIDを持つ。各stepはCLI採番のID、stage、statusを持つ。CurrentStepIDが実行を指定し、補助Stageとの不一致を拒否する。採番済みIDは却下した変更案を含め再利用しない。
- 新Intentにはinitializationとdiscoveryの2実行を未完了で作る。Approvedがない間はカタログ必須prefixだけを実行できる。これは人間が計画を承認済みという意味ではない。discovery終了までに任意4段階の採否・省略理由を承認する。
- 計画hashはID、stage、順序、採否、変更理由、やり直し対象を含む。進捗や承認自体は含めず、承認操作だけでhashが変わらないようにする。成果承認は計画版、実行回、レビュー対象、定義hashに結び付ける。
- Entry、Sensor、review、Accepted、文書宣言、テスト結果、Unitの割当・完了・統合結果をstep_idに対応させる。同名ステージの過去の成功を新実行へ使えない。Acceptedはstage名でなく実行回IDで保持し、旧4件上限を取り除く。既存の保存容量制限は維持し、上限超過は明示エラーにする。
- 承認済み計画の未完了の先頭だけをbeginできる。finishは現在回の終了検査・独立レビュー・成果承認が揃った時だけ完了させる。最終選択回ならIntentを完了する。integrationの選択は必須ではない。
- 変更案は承認済み計画と分けて保存する。追加・省略・並べ替えは毎回承認待ち。承認時に完了済みprefixを保持し、条件が変わった現在回の開始・合格・承認を失効させる。稼働workerは既存の停止確認を完了してから変更する。

## 公開CLI

```text
intent plan ID --space SPACE
intent plan ID --space SPACE --expect REV --file PLAN.json
intent plan-approval ID --space SPACE --expect REV --file DECISION.json
intent approval ID --space SPACE --expect REV --file DECISION.json
intent finish ID --space SPACE --expect REV
intent reopen ID --space SPACE --expect REV --step STEP_ID --reason TEXT
```

PLANはreason、steps、omitted、必要な場合reopen_step_idを持つ厳密なJSON。既存実行はidとstage、新規はstageのみを指定してCLIがIDを採番する。statusを利用者の入力にしない。必須prefix変更、完了実行の消去、IDとstageのすり替え、ID重複、採否漏れを拒否する。

DECISIONはrequest_id、target、decision（approve/reject）、session、turn、quoteを持つ。UserPromptSubmitで承認待ち作成後に受け取った実際の回答と照合する。同一回答で計画と成果を承認する場合は両方が既に提示された承認対象であることを必要とする。begin/check/procedure/reviewは現在回を使い応答に実行回IDを示す。文書宣言・テスト結果にはstep_idを必須としstageとの整合を検査する。旧advanceにはfinishへの案内付きエラーを返す。helpに型・状態・省略理由・実際のJSON例を載せる。

## Sensorと手順

initializationはSpace・Rule・workflow・CLI配置を確認しダミー成果文書を作らない。discoveryは要件と計画採否を確認し、資材があるだけで構成分析文書を要求しない。architecture-analysisは共有codekb/current-analysis.mdとarchitecture.mdを扱う。

planningは範囲・手順・検証を具体化する。planning省略のtddでも承認済み範囲・受入条件・テスト方法と実測結果は必要だが、特定の計画文書を一律には要求しない。tdd省略のintegrationでも対象commit・実測command・結果を要求する。integration→tddも選択可能。procedureが示すinputsとSensorの条件を一致させる。outputsは文書だけで空も許容する。OKF採用path/hashの保存と鮮度判定を維持し、文書全量snapshotは作らない。

## 承認・やり直し・保存失敗

hookは承認待ちと順序違反の通常AI作業を拒否する。読取り、help、質問回答、計画整理、正規承認、必要なレビューは許可し、discoveryで計画承認と成果承認が互いを待つ行き止まりを作らない。

Issue146の未マージ実装は参考資料として選択的に利用する。会話の本人認証やOSの全経路封鎖は保証しない。過去turnのA→B→A再送、別request・別実行・別計画版の回答転用を拒否する。再送判定用の最小ID/hashのみを容量制限付きで保持し、全prompt監査は作らない。JSONエスケープ後の保存容量を検査してからruntimeへ書く。

状態変更履歴は計画・承認・進捗の変更を対象にし、操作全件のauditにはしない。履歴のstatus・approval値・対象等も厳密に検査する。履歴を書いてstateのheadを確定するまで旧状態を正とし、途中保存失敗後も同じ操作を安全に再試行できる。回答出典をstate保存成功前に消費・破棄しない。

reopenは実行回を指定し、その回と必要な後続の再実行を変更案にする。承認前には適用しない。旧完了実績を保持し新しい実行回を作る。理由はknowledge/log/ID-work-log.mdに残す。既存のpending、前後hash、時刻固定、同一retryの契約を計画版と実行回へ結び直す。ADRは設計変更の理由に使う。

## 所有範囲と作業単位

1 Issue/PR、1 writer、work_unit_id=intent-execution-plan。対象はsrc/internal/workflow、flow、minimal、cli、install、workspaceと関連test、src/cmd/aidlcのjourney・承認実測test、src/core/workflowのカタログと6手順、src/coreのRule/WORKFLOW、harness内の配布参照、docs/developmentの現行説明、関連設計とRAM。必要なfixtureは新しい正規契約に合わせ、gateを除いて通さない。

元checkout /Users/const/sori883/ai-dd は読取り専用の参考。編集は /Users/const/sori883/ai-dd-stage-okf-documents のみ。Go単一binary・標準ライブラリを継続する。CLIによるサブエージェント起動は追加せず、既存の調整役と4担当の分担を維持する。

## 順序付きTDDと受入条件

各項目はtest先行で意図した失敗を実測してから実装する。型/signature不足のみcompile用の宣言と明示未実装エラーを足せる。既存・先行実装で成功する項目はALREADY_GREENと正直に記録する。

1. Schema: カタログ6種、prefix、新state保存、旧schema拒否、不正ID/採否拒否。`go test -count=1 ./src/internal/workflow ./src/internal/flow -run '^TestExecutionPlanSchema'`
2. Bootstrap: 新規作成、初期化begin、順序飛ばし拒否、既存Space再利用。人間承認を要する完走は項目5以降。`go test -count=1 ./src/internal/flow -run '^TestExecutionPlanBootstrap'`
3. Draft: 初回/変更の保存、完了prefix保持、却下、選択漏れ、稼働Unit停止。`go test -count=1 ./src/internal/flow -run '^TestExecutionPlanDraft'`
4. Evidence: 選択制Sensor、文書/結果/Unitの実行回対応、同stage再実行で過去合格拒否。`go test -count=1 ./src/internal/flow -run '^TestExecutionPlanEvidence'`
5. Approval: 回答照合、旧turn再送拒否、計画と成果の別対象、初期化→discoveryだけの完走、任意順完走。`go test -count=1 ./src/internal/flow -run '^TestExecutionPlanApproval'`
6. ReopenHistory: 変更承認後の再実行、履歴列挙と不正値拒否、回答/履歴/state/logの保存失敗と同一retry。`go test -count=1 ./src/internal/flow -run '^TestExecutionPlanReopenHistory'`
7. CLI: 型とhelp、正規承認操作、待機中hook、A→B→A再送とエスケープ後容量超過。`go test -count=1 ./src/internal/cli ./src/internal/minimal -run '^TestExecutionPlanCLI'`
8. Distribution: fresh配布と6手順の整合、CLI journey fixture、実機用の証拠validatorを更新。`go test -count=1 ./src/internal/install ./src/cmd/aidlc -run '^TestExecutionPlanDistribution'`。loopは対象unit testのみ、配布E2Eの実行はfinalに集約する。

work unit末尾は上記targeted群、変更したpackageの必要な既存test、gofmt適用、diff checkをまとめて確認する。全体検証は行わない。

独立review後、固定HEADで親がread-only finalを1回開始する。全package test、race、vet、gofmt -l、go mod tidy -diff、git diff --check、integration tagの結合journey、darwin/linux/windows×amd64/arm64の6build、native help/versionを確認する。導入済みcodex-cli 0.153.4による限定した実hook/会話承認journeyも実測し、モデルの回答内容の偶然に依存せず境界を検証する。実測前に試験用回答・承認であることを明示する。最終差分のGitHub checks成功後にPRをmergeし、mainとIssue closeを確認する。

## 許可と本家参照の境界

固定4段階から利用者が選ぶ実行列への変更、および会話承認・進捗履歴はユーザーが直接指定した製品設計である。本家の配布・Space作成を変更するものではなく、旧33 Stageの互換性を約束しない。最新upstreamとの一致を未確認のまま主張しない。Issue146の人間承認は本変更の実行回契約へ統合し、その受入条件も確認できた時点で完了にする。
