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
- `units`: 後述のUnit配列。分割しない場合は空配列とし、`tests` に直接実装の検証コマンド、
  `direct_commit` に検証したコード版を記す。検証前の結果を作らない。

理解段階では目的・現状・制約を確認し、必要なら調査、試作、質問を行う。
質問待ちは `A intent wait ID --space SPACE --expect R --reason TEXT --resume-condition TEXT`。
回答後は `A intent resume ID --space SPACE --expect R --reason TEXT`。
中断は `intent pause`、中止は `intent cancel` とし、同じID/Space/期待revision/理由を渡す。
段階を戻す場合は `A intent reopen ID --space SPACE --expect R --reason TEXT --stage STAGE`。

## Sensorと独立レビュー

`A intent check ID --space SPACE` で実ファイルを検査する。各境界と完了には現在のSensorと
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
