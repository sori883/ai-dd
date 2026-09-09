# 4段階の開始・終了Sensor実装計画

状態: Accepted scope。ユーザーはファイル別の開始/終了Sensor表を確認し「はい、実装してほしいです」と直接依頼した。
基準mainは76882f0d2acb5d0ff6bb031e26bc8f1736291cbb（PR #141）。

## 背景と利用結果

現在のGo CLIには終了条件と独立レビューを照合する仕組みがあるが、作業開始時の入力確認がなく、
任意のKnowledge一枚でも文書の存在条件を満たせる。今回Intentの要件・計画とSpace共有の現状解析・構成図を
区別して検査し、必要な入力を確認してから作業を開始し、成果と独立レビューが揃った場合だけ次へ進める。
Sensorは機械的な形式・関連付け・版の検査、reviewerは内容と実測根拠の検査を担当する。

## 合意したファイルと段階

次のパスはaidlc/spaces/<space>/knowledge/からの相対。

| 段階 | 開始 | 終了 |
| --- | --- | --- |
| discovery（目的整理＋深掘り） | rules/rule.md、存在する共有解析/構成図 | design/<intent_id>/requirements.md。既存資材がある場合はknowledge/current-analysis.mdとknowledge/architecture.md |
| planning（実装計画） | Rule、合格済requirements、参照する共有文書 | design/<intent_id>/implementation-plan.md、必要ADR、Unitの担当/依存/検証 |
| tdd | Rule、合格済requirements/plan、関連ADRと共有文書 | 実装・test・実行結果、変更判断のADR |
| integration（統合検証） | 合格済実装/test結果、requirements/plan/関連ADR、Rule | knowledge/<機能名>.md、共有解析/構成図、必要ADR、統合検証結果 |

初回discovery開始は共有解析/構成図の不存在を許す。既存資材がない場合も、統合終了時には実装後の現状を両文書へ記す。
共有文書は作業に必要なものを参照し、新しいIntentごとの複製・intent_id書換え・日時だけの更新を要求しない。
要件と計画はpathとfrontmatterのintent_idが今回Intentに一致することを要求する。
ADRはknowledge/ADR配下。必要/不要は既存config.adrで宣言し、不要理由もreviewerが確認する。

## 文書の最小契約

OKFとして解析でき、type/title/descriptionと非空本文を持つことを確認する。任意の見出しを大量に固定せず、
役割の中心となる次の第2階層見出し（##）と非空内容を最低条件にする。内容の十分性はreviewerが確認する。

| ファイル | type | 必須節 |
| --- | --- | --- |
| requirements.md | Requirements | 目的、範囲、要件、受入条件、未確定事項 |
| implementation-plan.md | ImplementationPlan | 変更箇所、実装手順、検証方法 |
| current-analysis.md | CurrentAnalysis | 現状、構成・動作、根拠、未確認事項 |
| architecture.md | Architecture | 構成図、構成要素、データフロー |
| knowledge/<機能名>.md | Knowledge | 機能、利用手順、制約 |
| ADR/<判断名>.md | ADR | 既存ADR契約と必要宣言を維持 |

未確定/未確認事項がなければ「なし」と明示できる。architectureの構成図節には非空Mermaid fenceを要求する。
図の構文・意味の完全検証器は作らず、reviewerが実物と照合する。Ruleは既存OKFと必須Rule読込み契約を維持する。
標準OKF metadataの検査を使い、generatedがある場合は既存日時形式検査を維持する。工程開始日時を新設しない。
必須型はこのSensorの文書契約であり、memory create/update全体の自由なtypeを制限するものではない。
本文例とCLI作成例をWORKFLOW/helpへ記載し、本文はAI、frontmatterは既存memory CLIが生成する。

## CLIと開始gate

- intent check ID --space SPACE --boundary start|end: 読取り専用の検査。省略時は従来どおりend。
- intent begin ID --space SPACE --expect REV: 開始検査に合格した入力版を保存して作業開始。
- intent review assign: 開始済みで終了Sensorが合格した現在対象を独立reviewへ割り当てる。
- intent review accept: 割当identityと現在targetを検証し、実報告のpass/failを受理する。
- intent advance: 終了Sensorと現在対象の独立reviewの両方が合格した場合だけ次へ進める。

beginはIntent lockと既存revision比較保存を使う。未開始で成功すればrevisionを一回増やす。
開始済みの同段階への再beginは期待revisionを確認して現在stateを返し、入力版を勝手に差し替えない。
開始不合格でもIntent作成/選択、help、Rule/WORKFLOW/OKF読取り、専用draft編集、configure、質問待ち/中断/再開は可能。
同じSpaceの正規memory create/updateによる必要な文書修復も可能。対象は固定文書、宣言済み共有/ADR文書とし、
任意shellや別Spaceへの書込みを修復扱いにしない。一般の実装/test操作とUnit claimは開始済みを要求する。
従来hookが扱う通常操作の漏れ防止であり、OS上の全経路を禁止する保証には拡張しない。

開始Sensorへ終了Sensorの全コードhashや成果完成条件を流用しない。開始成功後のコード/共有文書の正当な編集を
毎toolで拒否しない。前段合格requirements/planの変更は旧合格として利用せず、対応段階へreopenして再検証する。
一般toolとUnit claimの直前、および終了検査ではCheckWorkでactive/現stageのentry/前段不変入力を確認する。
Entry.Inputs全部、共有更新文書、現段階成果、全コードhashを不変入力として固定しない。
Ruleの変更は既存の当該turn読込み/hash確認で検知する。開始後のRule変更も最終reviewの対象に含める。

## stateと入力資材

新規Intentはschema_version=2。旧schemaは明示エラーとし、移行・自動補完・削除を行わない。
これは既存Intentを無視してよいというユーザー合意に従う。既存共有文書の内容は自動上書きしない。

Configへ追加:

- material_sources: 明示したrepository相対のUTF-8ファイル/ディレクトリ配列。空ならno_materials_reason必須。
- no_materials_reason: 既存資材がない理由。初回入力準備中は未設定を保存できるがdiscovery終了までに宣言する。
- feature_knowledge: integration終了のknowledge/<機能名>.mdのrepository相対path配列。最低1件、共有解析/構成図とは別。
- test_results: 作成済みの実行結果JSONのrepository相対path配列。stageはJSON内で区別し、段階ごとに追記する。

MaterialSourcesは明示対象だけを安定順序で列挙する。.git内部、state/runtime、symlinkや非regular、非UTF-8の対象を
診断し、黙って解析済みにしない。directoryの追加/削除もhash対象にする。範囲外を自動探索しない。
外部URL/PDF等はAIが利用可能な読取り手段で調査できるが、今回のGo Sensorの資材bytes検査対象はローカルUTF-8。
OKF sources.resourceは標準意味を保持する。独自のrepo相対意味へ上書きせず、文書根拠とmaterial_sourcesの意味上の対応は
reviewerが確認する。Sensorは明示された資材と文書の現在bytesを対象に含め、review後の変化を拒否する。

Stateへ追加:

- entry: nullまたは{stage, inputs:[{path,sha256}], sources:[{path,sha256}]}。現段階の開始時参照版だけ。
- accepted: stageをkeyとする{stage,review_target,outputs:[{path,sha256}]}。各段階の直近合格、最大4件。

entry/acceptedはCLI所有でconfigureでは変更できない。path/型/hash/重複/段階を厳密に検証する。
本文snapshot、全操作audit、receipt台帳、別の工程体系は追加しない。合格結果自身をdigestへ入れる自己循環を避ける。
開始入力のうち共有解析/構成図は現在段階でも更新でき、終了時には更新後の版を検査する。
前段の不変入力（requirements/plan、integration開始で使うtdd成果証拠）は対応acceptedと照合する。

configureで通常の現段階成果・計画を更新してもentryを毎回消さない。material_sources/no_materials_reason変更は
解析前提変更なのでdiscoveryではentryを無効化、後段ではdiscoveryへのreopenを要求する。
advanceは終了Sensor/reviewを再照合し現在の成果版をacceptedへ保存して次段階entryを未確認にする。
pause/resumeはentryを保持。reopenは対象段階のentryとその段階以降のacceptedを無効化しbeginから再開する。

## 実行結果の検査境界

TestResultsはstrict JSONで次の形式とする。

```json
{"stage":"tdd","runs":[{"command":"go test -count=1 ./target","commit":"40桁のGit commit","exit_code":0,"output_path":"evidence/test-output.txt"}]}
```

stageはtdd/integration。commandは非空、exit_codeは必須整数（省略を0とみなさない）、commitは実在して
現在の直接成果/Unit成果/統合成果と対応すること。output_pathはrepository相対の、安全な実在する非空ファイル。
TDD結果はdirect/Unit成果commitへ対応させ、同じcommandでも各Unit成果版の成功を要求する。
integration結果は全Unit統合後の現在HEADへ対応させる。過去TDD結果を後段HEADへ照合し直さない。
各段階のacceptedへはその段階の結果JSON/outputだけを含め、別段階の証拠を混入させない。
コードcommit後に証拠を作成し、証拠は未commitでも検査/reviewできる。証拠自身のcommitを成果commitへ強制しない。
該当段階の計画検証commandを成功結果で満たす。失敗結果の併記は可能だが失敗だけで合格にしない。
追加の検証commandも証拠として受理し、同じ形式・版・output条件で検査するが、必須の計画検証の不足を補うものにはしない。
Unit分割時は既存どおりUnit.testsを使い、直接実装用config.testsを新たなglobal必須条件として追加しない。
JSONとoutputとコードの現在bytesをreview digestへ含め、提出後の書換えを検知する。digestを証拠自身へ埋め込まない。
証拠は例えばaidlc/evidence/<space>/<intent_id>/配下へ保存でき、state/runtimeを成果物として使わない。

これは担当が提出した証拠の形式と版の検査であり、ログの真正性を認証する機構ではない。
REDの意味、実際のコマンド実行、未commitコードを含む実測対象、十分性は独立reviewerが確認する。
Go CLIにshell実行engineを追加しない。前表のtestファイル有無だけで実行成功とする説明をしない。

## 対象file、単独writer、TDD

1 Issue/PR、work_unit_id=start-end-sensors。直接承認内の各項目を一つのwriterへまとめる。
所有: src/internal/flow、src/internal/cli、src/internal/minimal、src/cmd/aidlcの関連test、
src/harness/codex/minimal/WORKFLOW.md、src/core/minimal/knowledge/rules/rule.md、docs/development.md、実装証拠RAM/索引。
SKILL原稿と4agent定義は既存の全文手順読込/役割で足りるため据置き、既存relocateの原稿判別を維持する。
既存fixtureは新しい必須条件を満たすよう更新し、検査をskipや弱化で回避しない。

scaffold: Boundary string、BoundaryStart/End、FileVersion/StageEntry/StageAcceptanceと上記Config/State field、
Store.CheckBoundary(id string,boundary Boundary)(Gate,error)、Store.Begin(id string,expect uint64)(State,error)、
Store.CheckWork(id string)error（active/開始済み/前段不変入力の共通検査）。
既存Checkは終了検査へ委譲。新宣言のcompile-only空実装を許し、runnable RED後に振る舞いを実装する。

順序付きtest-first:

1. 保存契約/CAS/旧schema非補完: go test -count=1 ./src/internal/flow -run '^TestBoundaryStore'
2. 開始表/初回不存在/合格版: go test -count=1 ./src/internal/flow -run '^TestStartSensor'
3. 終了文書/ID/type/節/共有版/実行証拠: go test -count=1 ./src/internal/flow -run '^TestEndSensor'
4. begin/advance/resume/reopen: go test -count=1 ./src/internal/flow -run '^TestBoundaryTransition'
5. review対象/前段合格/更新後旧pass拒否: go test -count=1 ./src/internal/flow -run '^TestBoundaryReview'
6. CLI/help/hook修復/Unit開始gate: go test -count=1 ./src/internal/cli ./src/internal/minimal -run '^TestBoundary'
7. 実CLI journeyと従来fixtureを更新。loopでは対象の通常testのみ実行し、integration/live実行はfinalへ集約。

末尾は上記targeted群とaffected package（flow/cli/minimal/install/cmd）の通常test、gofmt、diff check。
親は末尾で全差分とtargeted群を一度確認し、独立reviewへ渡す。

## 受入とfinal

- 初回Ruleのみでdiscovery開始可能。不足成果は終了拒否。修復CLIは使える。
- 他Intentの要件/計画、任意Knowledgeでの代用、不正型/空節/図欠落を拒否。
- 共有文書に別IntentID/古い有効日時があっても、それだけを理由に拒否しない。
- 正当なコード・共有文書更新を止めず、前段合格入力変更や旧reviewの再利用は拒否。
- begin再試行/CAS/保存失敗/再開/段階戻し/Unit開始を整合させる。
- 非空ログだけ・失敗だけ・違う成果版では終了不可。実CLIで4段階を完走。

独立review後、対象ファイルを変更しないfinalで全test、race/shuffle、vet、tidy-diff、format/diff、
全integration、darwin/linux/windows×amd64/arm64 build、native fresh配布を実施する。
固定Codex0.153.4の限定liveで未開始拒否→入力準備/修復→begin→通常編集を観測する。
実CLI journeyの4段階完走と、限定liveの保証範囲は区別する。

## 許可と互換性

実装許可は開始/終了表への直接依頼。Go単一バイナリ、外部Go module追加なし。
固定AI-DLC2.6.123の33Stage/receiptを再導入せず、合意済み4段階に開始と終了の機械検査を置く意図的な構成を採用する。
共有Knowledgeの参照版とIntent成果を分け、記録のための作業日誌を作らないという依頼を守る。
旧Intent移行/二重運用は不要。変更はGitで戻せるが、利用先dataや配布済み設定を自動上書き/巻戻ししない。
重要な未決の仕様差分が発生した場合だけユーザーへ戻す。独立review/final/GitHub checks後に通常merge commitで自律マージする。
