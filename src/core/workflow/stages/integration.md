---
stage_id: integration
agents:
  - role: independent_review
    agent: aidlc-reviewer
inputs:
  - match: {type: Rule, title: "四段階の作業合意"}
    count: one
    version: current
  - match: {type: CurrentAnalysis}
    count: optional
    version: current
  - match: {type: Architecture}
    count: optional
    version: current
  - match: {type: Requirements, intent_id: "${intent_id}"}
    count: one
    version: accepted
    accepted_at: discovery
  - match: {type: ImplementationPlan, intent_id: "${intent_id}"}
    count: one
    version: accepted
    accepted_at: planning
  - declared: intent_documents
outputs:
  - role: current_analysis
    path: "${knowledge_root}/knowledge/current-analysis.md"
    metadata: {type: CurrentAnalysis}
  - role: architecture
    path: "${knowledge_root}/knowledge/architecture.md"
    metadata: {type: Architecture}
  - declared: intent_documents
sensors:
  start: integration-start
  end: integration-end
---

# 統合検証

合格済み要件・計画・TDD結果が変わっていないことを確認する。
全Unitを統合し、最終HEADで全計画commandを再実行する。追加統合commandも同じHEADの実測結果を提出できる。
config.test_resultsへ現在存在するintegration結果JSONを追記する。TDD結果を上書きしない。
共有解析と構成図を現行実装へ更新する。type CurrentAnalysisは## 現状・構成・動作・根拠・未確認事項（「構成・動作」は一つ）。
type Architectureは## 構成図・構成要素・データフロー、構成図には非空mermaid fenceを置く。
共有解析/図とは別に、knowledge/FEATURE.mdへtype Knowledge、## 機能・利用手順・制約を保存し、intent documents outputsへ最低1件を宣言する。
必要ADRを照合する。実装修正はtdd、計画変更はplanning、要件変更はdiscoveryへreopenする。
最終HEAD・実測結果・利用者の受入を揃えて独立レビューへ渡す。完了位置はgraphが決める。

入口の実行ファイルを `A`、Intentを `ID`、Spaceを `SPACE` とする。
`intent show`のrevisionを `R` とし、変更は `--expect R`。競合時は再読込する。stateを直接編集しない。
各turnでbindが返す必須Ruleを全文読む。現在手順のinputsを読み、
`A intent check ID --space SPACE --boundary start` と `A intent begin ID --space SPACE --expect R` を通してから作業する。
未開始でも正規CLIで必要文書を修復できる。本文草稿はSessionStartのdraftパスへ作り、metadataはmemory CLIへ渡す。
引数・型・値とJSON例は `A intent configure --help`、`A memory create --help`、`A memory update --help` を参照する。
段階変更・再開後は `A intent procedure ID --space SPACE` で現在手順を取り直す。
質問待ちはwait、回答後resume。差戻しは `A intent reopen ID --space SPACE --expect R --reason TEXT --stage STAGE`。
理由はwork-log.mdにCLIが保存する。保存失敗は同じexpect・元/先段階・理由でretryし、記録改変時は元版を復元する。
定義変更時は元版を復元するか新Intentにする。差戻し理由だけでADRを作らない。

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
generatedの日時は自動。出典・検証の日時を捏造せず、必要なmetadata変更だけ引数で渡す。ADRは `adr/NAME` とtype adrを使う。
stateのADR参照は `aidlc/spaces/SPACE/knowledge/adr/NAME.md`。記録成功や検証完了を未実施のまま主張しない。
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

## 入出力文書の登録

`intent procedure ID --space SPACE` の inputs は metadata の完全一致で解決する。count は one=1件、optional=0〜1件、many=1件以上。tags は順序なし集合。accepted は accepted_at の同じ path/hash を保持し、変更にはその段階への reopen が必要。共有 current 文書は更新できる。
`intent documents ID --space SPACE` で明示一覧を読み、`intent documents ID --space SPACE --expect REV --file DOCUMENTS.json` で inputs/outputs 両配列を一括置換する。各要素は stage、Space内の具体的な Markdown path、metadata(type/title/description必須、status/tags/intent_id任意)。未存在の outputs は宣言できるが、終了までにその path と metadata で保存する。追加の成果文書も必ず outputs に登録する。受入済み段階の宣言変更は先に reopen する。
新規 adr の folder/type は小文字で、現在 Intent ID は登録時に自動保持する。既存の共有文書や過去の adr は意味が変わらない限り ID や日時を付け直さない。採用した過去 adr は inputs に具体 path で登録する。config.adr は required/reason のみ。
統合では共有解析・構成図に加えて機能 Knowledge を最低1件 outputs へ登録する。文書成果がない TDD は outputs を空にできる。test_results は別契約で、実行済みの存在する JSON/output だけを configure へ登録し、将来段階の結果を事前登録しない。
