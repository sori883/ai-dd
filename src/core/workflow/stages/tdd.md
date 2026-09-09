---
stage_id: tdd
agents:
  - role: implementation
    agent: aidlc-worker
  - role: independent_review
    agent: aidlc-reviewer
inputs:
  - path: "${knowledge_root}/rules/rule.md"
    version: current
  - path: "${knowledge_root}/design/${intent_id}/requirements.md"
    version: accepted
    accepted_at: discovery
  - path: "${knowledge_root}/design/${intent_id}/implementation-plan.md"
    version: accepted
    accepted_at: planning
  - path: "${knowledge_root}/knowledge/current-analysis.md"
    version: current
    required_when: exists
  - path: "${knowledge_root}/knowledge/architecture.md"
    version: current
    required_when: exists
  - refs: config.adr.refs
    version: current
    required_when: adr_required
outputs: []
sensors:
  start: tdd-start
  end: tdd-end
---

# TDD

合格済要件と実装計画、必要ADRを読む。承認済み計画の範囲でaidlc-workerへ実装を依頼する。
テストを先に追加してrunnableな失敗を観測し、最小実装、成功確認、整理の順で繰り返す。
テスト不在・skip・compile failureを意図したRED/GREENと同一視しない。
テストの修正反復はtdd内で行える。計画変更はplanning、要件変更はdiscoveryへreopenする。
実装後は実測command・成果commit・exit_code・output_pathをtest_resultsへ登録する。UnitごとのResultCommitで成功が必要。
直接実装はdirect_commitとtestsを揃える。必要な変更判断のADRは保存するが、文書成果がなければoutputs=[]でよい。
コード・テスト・ADR要否の検査は省略しない。次のintegration結果を事前作成・宣言しない。

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
generatedの日時は自動。出典・検証の日時を捏造せず、必要なmetadata変更だけ引数で渡す。ADRは `ADR/NAME` とtype ADRを使う。
stateのADR参照は `aidlc/spaces/SPACE/knowledge/ADR/NAME.md`。記録成功や検証完了を未実施のまま主張しない。
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
