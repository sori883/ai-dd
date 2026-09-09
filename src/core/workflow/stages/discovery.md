---
stage_id: discovery
agents:
  - role: research
    agent: aidlc-researcher
  - role: requirements
    agent: aidlc-requirements
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
  - declared: intent_documents
outputs:
  - role: requirements
    path: "${knowledge_root}/design/${intent_id}/requirements.md"
    metadata: {type: Requirements, intent_id: "${intent_id}"}
  - declared: intent_documents
sensors:
  start: discovery-start
  end: discovery-end
---

# 目的整理と深掘り

目的、範囲、受入条件、現在の動作と制約を理解する。結果を左右する判断はユーザーへ質問する。
必要な一次資料の調査はaidlc-researcherへ、根拠からの要件整理はaidlc-requirementsへ依頼する。
事実・推測・未確認を区別し、実装計画を妨げる未確定事項だけをconfig.unknownsへ残す。
子担当は本文案を返し、共有state/OKF文書は調整役だけが保存する。
既存資材はconfig.material_sourcesへ明示しUTF-8を確認する。資材がない場合は終了までにno_materials_reasonを書く。
新規projectはRuleだけでbeginできる。Git未初期化ならbegin後にgit initを行う。
既存資材があるとき共有解析/図を更新する。なければ開始時のダミー文書は不要。
要件はdesign/ID/requirements.md。type Requirements、intent_id ID、## 目的・範囲・要件・受入条件・未確定事項を非空にする。
共有解析はknowledge/current-analysis.md、type CurrentAnalysis、## 現状・構成・動作・根拠・未確認事項（「構成・動作」は一つの見出し）。
構成図はknowledge/architecture.md、type Architecture、## 構成図・構成要素・データフロー。構成図は非空mermaid fence。
共有文書へIntentIDや日時だけの変更をしない。要件・コードHEAD・configを揃えて終了検査へ進む。

入口の実行ファイルを `A`、Intentを `ID`、Spaceを `SPACE` とする。
`intent show`のrevisionを `R` とし、変更は `--expect R`。競合時は再読込する。stateを直接編集しない。
各turnでbindが返す必須Ruleを全文読む。現在手順のinputsを読み、
`A intent check ID --space SPACE --boundary start` と `A intent begin ID --space SPACE --expect R` を通してから作業する。
未開始でも正規CLIで必要文書を修復できる。本文草稿はSessionStartのdraftパスへ作り、metadataはmemory CLIへ渡す。
引数・型・値とJSON例は `A intent configure --help`、`A memory create --help`、`A memory update --help` を参照する。
段階変更・再開後は `A intent procedure ID --space SPACE` で現在手順を取り直す。
質問待ちはwait、回答後resume。差戻しは `A intent reopen ID --space SPACE --expect R --reason TEXT --stage STAGE`。
理由は`aidlc/spaces/<space>/knowledge/log/<intent_id>-work-log.md`にCLIがOKF（type: work-log）で保存する。保存失敗は同じexpect・元/先段階・理由でretryし、記録改変時は元版を復元する。
stateのrevisionが要求revisionを超えて確定するまで、記録の存在だけで成功扱いしない。generated.atはCLIが生成する内容更新時のUTC日時で、承認・検証済みを意味しない。
`A memory search work-log --space SPACE --intent-id ID`で検索し、`A memory show log/ID-work-log --space SPACE`で本文を読む。
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
## 入出力文書の登録

`intent procedure ID --space SPACE` の inputs は metadata の完全一致で解決する。count は one=1件、optional=0〜1件、many=1件以上。tags は順序なし集合。accepted は accepted_at の同じ path/hash を保持し、変更にはその段階への reopen が必要。共有 current 文書は更新できる。
`intent documents ID --space SPACE` で明示一覧を読み、`intent documents ID --space SPACE --expect REV --file DOCUMENTS.json` で inputs/outputs 両配列を一括置換する。各要素は stage、Space内の具体的な Markdown path、metadata(type/title/description必須、status/tags/intent_id任意)。未存在の outputs は宣言できるが、終了までにその path と metadata で保存する。追加の成果文書も必ず outputs に登録する。受入済み段階の宣言変更は先に reopen する。
新規 adr の folder/type は小文字で、現在 Intent ID は登録時に自動保持する。既存の共有文書や過去の adr は意味が変わらない限り ID や日時を付け直さない。採用した過去 adr は inputs に具体 path で登録する。config.adr は required/reason のみ。
統合では共有解析・構成図に加えて機能 Knowledge を最低1件 outputs へ登録する。文書成果がない TDD は outputs を空にできる。test_results は別契約で、実行済みの存在する JSON/output だけを configure へ登録し、将来段階の結果を事前登録しない。
