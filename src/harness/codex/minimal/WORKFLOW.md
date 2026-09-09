# 四段階の完全手順

入口skillで示した実行ファイルを `A`、選択した目的を `ID`、Spaceを `SPACE`、会話を `SESSION` と記す。
最初にbindの出力で現在stateと必須Rule全文を読む。以下は操作手順であり、Ruleの代わりではない。

## 状態と計画

`A intent show ID --space SPACE` のrevisionを `R` とする。変更は現在の `--expect R` で比較保存する。
競合時は再読込し、変更を検討し直す。強制上書きしない。JSON草稿はSessionStartのdraftパス、
または `aidlc/.runtime/` に置く。state.jsonを直接編集しない。

`A intent configure ID --space SPACE --expect R --file FILE` はconfig全体を置き換える。
既存fieldを保持し、Unitの進捗を偽装しない。configは次のfieldを使う。

- `objective`: 目的。`scope`: 範囲の配列。`acceptance`: 完成条件の配列。
- `unknowns`: 実装計画を妨げると明示した未確定事項だけの配列。全疑問ゼロを要求しない。
- `plan`: 実装計画。`code_revision`: 現在のGit HEAD。
- `adr`: `{required: bool, reason: string, refs: []}`。必要なADRだけ作り、不要なら理由を書く。
- `artifacts`: `{path, kind, stage}` の配列。pathはプロジェクト相対。kindはKnowledge/ADR/test。
  stageはdiscovery/planning/tdd/integration。将来段階の成果物は計画として定義できる。
  stateやruntimeを成果物にしない。Knowledgeは内容に応じたOKF typeを保持できる。
- `material_sources` / `no_materials_reason` / `feature_knowledge` / `test_results`: 後述の追加情報。
- `units`: 後述のUnit配列。分割しない場合は空配列とし、`tests` に直接実装の検証コマンド、
  `direct_commit` に検証したコード版を記す。検証前の結果を作らない。

理解段階では目的・現状・制約を確認し、必要なら調査、試作、質問を行う。
質問待ちは `A intent wait ID --space SPACE --expect R --reason TEXT --resume-condition TEXT`。
回答後は `A intent resume ID --space SPACE --expect R --reason TEXT`。
中断は `intent pause`、中止は `intent cancel` とし、同じID/Space/期待revision/理由を渡す。
段階を戻す場合は `A intent reopen ID --space SPACE --expect R --reason TEXT --stage STAGE`。

## Sensorと独立レビュー

開始は後述のstart検査とbeginで確認する。`A intent check ID --space SPACE` は終了の実ファイル検査。各境界と完了には現在の終了Sensorと
独立レビューのpassが必要。失敗、未実施、変更前の古いpassで進めない。

別root・別sessionのread-only reviewerを起動する。reviewerのcheckoutは調整rootと同じコード版・bytesにする。
共有Knowledge/ADRは調整rootの明示pathから読める。Rule全文、state、対象hash、成果物pathを渡し、実報告を受け取る。

`A intent review ID --space SPACE --expect R --file FILE` へ次のJSONを渡す。

```json
{"action":"assign","coordinator_session":"調整役session","session":"reviewer session","root":"reviewerの絶対root"}
```

返された対象hashを固定してreviewする。結果は実報告のsession/root/target/status/summaryをそのまま使う。

```json
{"action":"accept","session":"reviewer session","root":"reviewerの絶対root","target":"対象hash","status":"pass","summary":"根拠と具体的指摘"}
```

statusは実報告に応じて `pass` または `fail` とする。指摘は修正し、対象が変わったら再割当・再レビューする。`A intent advance ID --space SPACE --expect R` は一段階だけ進む。
統合検証後のadvanceで完了する。reviewerの報告を自分でpassへ書き換えない。

## Unitと並列作業

Unitは一つの担当作業、Boltは作業のまとまり。Unitの `id` は英数字で始まる英数字・`_`・`-` の80文字以内。
新Unitは `status: pending`、`result_commit` と `integrated_commit` は空にする。
`bolt`、`base_commit`、`depends_on`、`scope`、`tests` を計画する。配列fieldは省略せず保持する。
既存Unitの進捗は専用操作が管理し、configureで変更しない。

調整役AIがworkerを別worktreeへ起動する。共有stateのwriterは調整役一人。
依存Unitの統合commitを含むbaseから開始し、担当範囲とworktreeを重複させない。

`A unit claim ID --space SPACE --expect R --file FILE` には `{unit,session,root}` をJSONで渡す。
現在の割当は `aidlc/.runtime/flow/units/SPACE/ID/UNIT.json` にあり、`run_id` を確認できる。
workerは実行可能なテストのREDを観測してから実装し、同じテストでGREENを確認する。
成果commitができたら `unit result` へ `{unit,session,root,run_id,commit}` を渡す。
調整rootへ実統合してから `unit integrate` へ `{unit,commit}` を渡す。いずれもID/Space/期待revision/FILEを指定する。

中断したrunは実workerと現在commitを確認する。`unit confirm` へ同じrun identityと現在commitを渡してから再開する。
不明なrunを自動再実行しない。Go CLIは起動やGit統合を代行しない。実行環境で許可されたGit操作を使う。

## KnowledgeとADR

Knowledgeは現行のwhat/how、ADRはwhy・代替案・影響。毎操作の日誌や一律ADRは作らない。
草稿には本文だけを書く。frontmatterはCLIが引数から生成する。Concept IDは拡張子なしで、例は `knowledge/addition`。
引数・型・選択肢に迷ったら `A memory create --help` / `A memory update --help` を参照する。

```text
A memory create knowledge/NAME --space SPACE --body-file FILE --actor process:codex --type TYPE --title TITLE --description DESCRIPTION
A memory show knowledge/NAME --space SPACE
A memory update knowledge/NAME --space SPACE --body-file FILE --actor process:codex --expect HASH
A memory search QUERY --space SPACE [--intent-id ID]
```

showの `content` と `hash` を確認し、本文だけを草稿に書く。更新時の省略metadataはCLIが保持する。
generatedの日時は自動。出典・検証の日時を捏造せず、必要なmetadata変更だけ引数で渡す。ADRは `ADR/NAME` とtype ADRを使う。
stateのADR参照は `aidlc/spaces/SPACE/knowledge/ADR/NAME.md`。記録成功や検証完了を未実施のまま主張しない。

## 別cloneへ移ったとき

移転先のAI会話を開始する前に端末で `aidlc install codex --help` を確認し、
`aidlc install codex --relocate --project-dir NEW_ROOT --from-project-dir OLD_ROOT --from-binary OLD_BINARY`
を実行する。旧pathは配置済み参照と一致する絶対文字列を指定する。既存SKILL原稿の編集や版更新を兼ねない。
新しい `.codex/hooks.json` の絶対pathとCodex hook trustを利用者が確認する。認証設定を自動変更しない。
部分失敗のPaths/Pendingを読み、同じ引数で全件を再検査する。未知編集を推測で上書きしない。

旧workerの終了を確認し、`unit reassign --help` のJSONで同じIntent/Unitを新しい別worktree/sessionへ割り当てる。
runningなら既存pause/resumeでneeds_confirmationにする。`previous_run_stopped: true` は確認済みの場合だけ指定し、
CLI成功まで新workerを開始しない。runtimeはGit共有しない。保存途中は他のstate更新（configureや別Unit操作など）が拒否される。同じexpect/JSONで再試行し、
既にrunningならshowと現在assignmentで成功を確認する。新run_idと現在HEADで再テスト後result/integrateする。
レビューは新root/sessionへassignし、現在targetの独立reviewを受け直す。

## 4担当への委譲

利用者との対話と共有state・Knowledge/ADRの書込みは調整役一人が担当する。
必要な調査は `aidlc-researcher`、要件の具体化は `aidlc-requirements`、承認後の実装は
`aidlc-worker`、固定成果の独立レビューは `aidlc-reviewer` へ依頼する。
単純な仕事で毎回4担当を起動する必要はない。調査は独立した読取りを並列化でき、
実装の並列化は既存のUnit割当・依存・別worktree条件に従う。
要件整理は必要な調査結果の後、workerは承認と割当の後、reviewerは成果固定の後に起動する。

依頼は全履歴の継承を既定にせず、目的、Intent/Space、対象root、承認済み範囲、必要なRule全文・資料、
期待出力を含む短い自己完結した文章にする。researcherには質問と対象version・OKF検索結果、
requirementsには要望・承認条件・現状・調査結果、workerには計画・Unitまたは担当範囲・別worktree・base・検証方法、
reviewerには固定hash・同版の別root・state・実測結果を渡す。
read-onlyの3担当にはwriterのhookを持たない別root、workerには別worktreeを用意する。
子はcoordinator専用hook操作や共有state・OKFの更新を代行しない。
sandbox_modeの設定をOS全経路の保証と扱わず、実行環境とtoolの権限を守る。

researcherから根拠path/URL・参照版付きの事実/推論/未確認、相違点・選択肢・質問を受け取る。
requirementsから目的・利用者・範囲・要件・制約・受入条件・未確定事項と確認質問を受け取り、
必要なユーザー回答と承認は調整役が得る。workerから変更file・実行testと結果・成果commit（未作成ならその旨）・残件を受け取る。
reviewerから対象版・pass/fail・summary・優先度と根拠path付きfindingを受け取り、既存のreview受理CLIへ渡す。
追加調査や担当が必要なら子から調整役へ戻す。reviewerに広い新規調査を兼務させない。
子の報告や本文案は調整役が内容を確認し、必要なKnowledge/ADRをCLIで保存する。

## 段階の開始と終了

Sensorは必要ファイルの形式・Intent対応・版を機械検査する。内容と実測の妥当性は独立reviewerが確認する。
各段階で入力を用意して開始検査を行い、begin成功後に作業する。A/ID/SPACE/Rは実値へ置き換える。

```text
A intent check ID --space SPACE --boundary start
A intent begin ID --space SPACE --expect R
```

checkは読取り専用。beginは開始時の参照版を保存し、返されたrevisionを次の更新へ使う。
同段階の再beginは参照版を取り直さない。成果を用意したら終了検査を行う。

```text
A intent check ID --space SPACE --boundary end
```

boundary省略も終了検査。終了Sensor合格後にreview assign、実報告pass/failをaccept、
終了Sensorと独立reviewが現在版で合格したらadvanceする。次段階もbeginが必要。
開始前もhelp/read/configure/専用draft/同Spaceの必要文書の正規memory修復は可能。
一般実装・test・Unit claimは開始後に行う。Rule未読や待機中の書込みを修復例外にはしない。
pause/resumeは開始記録を保持。前段合格要件/計画を変える場合はその段階へreopenしbeginから再確認。
共有解析/図や現段階成果は更新できるが、変更前のreview passは再利用しない。
新規Intentはschema2。旧schemaは明示エラーで、手編集による版の付け替えや削除で回避しない。

## 必須文書

以下はaidlc/spaces/SPACE/knowledge/からの相対path。要件/計画のIDとfrontmatter intent_idは現在Intentに一致させる。

| 段階 | 開始 | 終了 |
| --- | --- | --- |
| discovery | rules/rule.md、存在する共有解析/図 | design/ID/requirements.md。既存資材があれば共有解析/図 |
| planning | Rule、合格済要件、参照共有文書 | design/ID/implementation-plan.md、必要ADR、Unit担当/依存/検証 |
| tdd | Rule、合格済要件/計画、関連ADR/共有文書 | 実装/test、実行結果、変更判断のADR |
| integration | 合格済実装/test結果、要件/計画/ADR、Rule | knowledge/機能名.md、共有解析/図、必要ADR、統合検証結果 |

共有解析はknowledge/current-analysis.md、構成図はknowledge/architecture.md。
初回discovery開始時は両文書がなくてもよい。新規開発でも統合終了までに実装後の現状を両文書へ記す。
共有文書をIntentごとに複製したり、intent_idや日時だけを更新したりしない。
ADRはADR/判断名.md。必要性と参照先はconfig.adrで宣言する。

## configへ追加する情報

次は追加fieldのみの抜粋であり、完全なconfigure入力ではない。showで現在configを確認し既存fieldを保持した全体JSONへ組み込む。
SPACE/IDとパスを実値へ置き換える。

```json
{
  "material_sources": ["src/service", "docs/specification.md"],
  "no_materials_reason": "",
  "feature_knowledge": ["aidlc/spaces/SPACE/knowledge/knowledge/search.md"],
  "test_results": ["aidlc/evidence/SPACE/ID/tdd.json"]
}
```

material_sourcesは今回調べるrepository相対UTF-8ファイル/ディレクトリ。推測で全件探索しない。
空ならno_materials_reasonを書く。初回beginは未宣言でも可能だがdiscovery終了までに宣言する。
解析前提を変えるとdiscoveryの開始確認が無効になる。後段ならdiscoveryへreopenする。
feature_knowledgeは共有解析/図とは別の現行機能説明で、統合終了時に最低1件必要。
型・入力条件に迷ったらintent configure/check/beginとmemory create/updateの正規--helpを読む。
OKF sources.resourceの標準意味を保ち、文書の出典と明示資材の意味上の対応はreviewerが確認する。

## 本文の作成

草稿は本文だけ。frontmatterはCLI引数から生成する。各commandの前に対象文書の本文をFILEへ用意する。

```text
A memory create design/ID/requirements --space SPACE --body-file FILE --actor process:codex --type Requirements --title '今回の要件' --description '目的と受入条件' --intent-id ID
A memory create design/ID/implementation-plan --space SPACE --body-file FILE --actor process:codex --type ImplementationPlan --title '今回の実装計画' --description '変更手順と検証方法' --intent-id ID
A memory create knowledge/current-analysis --space SPACE --body-file FILE --actor process:codex --type CurrentAnalysis --title '現状解析' --description '現在の構成と根拠'
A memory create knowledge/architecture --space SPACE --body-file FILE --actor process:codex --type Architecture --title '現在の構成図' --description '構成要素とデータフロー'
A memory create knowledge/FEATURE --space SPACE --body-file FILE --actor process:codex --type Knowledge --title '機能の使い方' --description '現行の機能と制約'
```

既存文書はmemory showで読んでからmemory updateする。HASHはshowの現在hash。

```text
A memory update design/ID/requirements --space SPACE --body-file FILE --actor process:codex --expect HASH
```

metadata省略時は既存値を保持する。type/intent_id修復が必要な場合だけ明示する。日時はCLIが生成する。
status stableは工程合格を意味しない。必須の第2階層見出しと非空内容は次のとおり。

| type | 本文の##見出し |
| --- | --- |
| Requirements | 目的、範囲、要件、受入条件、未確定事項 |
| ImplementationPlan | 変更箇所、実装手順、検証方法 |
| CurrentAnalysis | 現状、構成・動作、根拠、未確認事項 |
| Architecture | 構成図、構成要素、データフロー |
| Knowledge（機能説明） | 機能、利用手順、制約 |

未確定/未確認事項がなければ「なし」と書く。Architectureの構成図節には非空mermaid fenceを置き、実際の構成を描く。
これらの型はSensorの必須文書の契約で、memory CLI全体の自由なtypeを制限しない。

## テスト実行結果

担当が実際に実行した結果をJSONと非空の出力ファイルで提出する。以下の値は必ず実測値へ置き換える。

```json
{"stage":"tdd","runs":[{"command":"go test -count=1 ./target","commit":"実在する40桁の成果commit","exit_code":0,"output_path":"aidlc/evidence/SPACE/ID/tdd-output.txt"}]}
```

stageはtdd/integration、output_pathはrepository相対、exit_codeは必須整数。
計画の検証を成功結果で満たす。失敗結果は併記できるが失敗だけで合格しない。
TDD結果はdirect成果commit、Unit分割時は各UnitのcommandとResultCommitの組をそれぞれ満たす。
同じcommandを使う複数Unitでも、一方の成果版の成功で他方を代用しない。
integration結果は全Unit統合後の現在HEADで計画commandをすべて成功させる。追加commandも同じHEADを使う。
コードcommit後に証拠を作り、証拠が未commitでも検査/reviewできる。過去TDD結果を後段HEADへ書換えない。
test_resultsには現在存在する結果だけを指定する。上の例はtdd.json作成後の値。
integrationで実行しintegration.jsonとoutputを作成してから、既存tdd.jsonを保って配列へ追記する。
未来の未存在ファイルは事前宣言しない。別段階のJSONは形式検査し、その内容を現在段階のaccepted outputsへ混ぜない。
Sensorは形式・版・出力を検査し、証拠/コード変更をreview対象に反映する。Go CLIはshell実行engineやログ認証器ではない。
RED/GREEN実測、未commitコードを含む検証対象、テストの十分性を独立reviewerへ根拠とともに渡す。
