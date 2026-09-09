# OKF selector と具体的文書一覧の実装証拠

Issue147、work_unit_id=`explicit-stage-documents`、verification_mode=`loop`。開始HEADは `8b05f9cc79185459e43cf6a05c7da18aeeb605f3`、作業先は独立した `ai-dd-stage-okf-documents` worktree。直接承認と受入契約は [計画](../../design/stage-okf-documents-plan.md) と [承認記録](2026-09-09-stage-okf-selectors-approved.md)。人間承認Issue146を変更せず、4段階・advance・reopenを維持した。

## 実装した動作

metadata の完全一致AND・tags集合・one/optional/manyの件数で入力を解決する。現在Spaceの安全な実Markdownだけを読み、同じ collector の本文と hash を選択・節検査・material・accepted比較で共有する。開始時の選択pathを交換せず、共有current文書の更新を許す。任意の追加宣言を空にしても必須要件・計画・Rule・コード・テストの検査は残す。

schema4 の `document_inputs` / `document_outputs` は `intent documents` がCASで一括置換する。configureは一覧を保持し、一覧の直接入力と旧fieldを拒否する。過去の合格段階の宣言変更はreopen、現在のinput変更はbeginのやり直しを要求する。outputのみの追加でentryは消さない。新規adrのIntent IDは登録後も保持し、採用済みの過去adrを具体pathで検査する。

procedure は条件・候補・具体output・欠落診断を返す。hookは選択/Rule/activeの既存境界を維持し、宣言文書と未解決の必要inputを正規memory CLIで修復できる。helpと段階手順は未存在outputの宣言と、実在する実行結果だけを登録するtest_resultsを区別する。

## 順序付きTDD

ログは実行環境の `/tmp/stage-documents-*` に保存した。初期RED/GREENコマンドはログを表示するshellで実行したため、個別go testの数値exitを別ファイルには保存していない。REDはログの意図したassertionとFAIL、GREENはpackageのokで確認した。末尾検証は数値exitを個別保存する。

|slice|exact targeted command|RED / GREENログと理由|
|---|---|---|
|1|`go test -count=1 ./src/internal/okfmemory -run '^TestDocumentSelector'`|`1-red.log`→`1-green.log`: 欠落・件数・不正入力。`1b-red.log`→`1b-green.log`: FIFOを開いて停止する不具合を通常file検査で修正。|
|2|`go test -count=1 ./src/internal/workflow -run '^TestDocumentDeclaration'`|`2-red.log`→`2-green.log`、`2b-red.log`→`2b-green.log`: 新frontmatterと配布原稿。`2c-red.log`→`2c-green.log`: YAML数値が文字列へ暗黙変換される問題を型検査で拒否。|
|3|`go test -count=1 ./src/internal/flow -run '^TestIntentDocuments'`|`3-red.log`→`3-green.log`、`3b-red.log`→`3c-green.log`: 登録/CAS/保存失敗・entryと新規adrのID保持。`3d-red.log`→`3d-green.log`: 旧JSON field拒否。|
|4|`go test -count=1 ./src/internal/flow -run '^TestSelectedDocuments'`|`4-red.log`→`4-green.log`: Sensor接続、曖昧化/選択交換/具体output。`4c-red.log`→`4c-green.log`: 共有文書をmetadataで検査。`4f-red.log`→`4f-green.log`: 登録済み機能Knowledge。`4g-red.log`→`4g-green.log`: TDD proofを通常inputの消失と誤判定しない。`4h/4i-red.log`→対応green: 空の追加宣言で必須前提を迂回しない。|
|5|`go test -count=1 ./src/internal/flow -run '^TestLowercaseADR'`|`5-red.log`→`5-green.log`: 小文字adrの明示採用と未採用/旧大文字拒否。|
|6|`go test -count=1 ./src/internal/cli ./src/internal/minimal -run '^TestIntentDocuments'`|`6b-red.log`→`6-green.log`: 厳密CLI・一覧保持。`6c-red.log`→green: read/宣言output修復。`6d-red.log`→`6e-green.log`: 登録前の必要input修復と別Intent拒否。`6f-red.log`→green: procedure解決結果。|
|7|`go test -count=1 ./src/internal/install -run '^TestDocumentDistribution'`|`7c-red.log`→`7-green.log`: 小文字path/typeと手順。`7e-red.log`→green: fresh配布の必須Rule selector。|

`4b` の資材なし共有文書pathと `4d` のcollector cache回帰は ALREADY_GREEN。資材ありの必須条件を追加した `4c` が有効REDであり、成功済み実装を戻してREDを演出していない。

## fixture訂正と通常詳細

- `3b-green` の過去合格fixtureにReviewTargetが欠けていた。正式な64桁値へ訂正した。目的の過去宣言拒否assertは初回到達でGREEN。
- `6-red` のminimal fixtureは絶対draft pathを相対file helperに渡していた。通常os.WriteFileへ訂正した `6b-red` が有効RED。
- `7-red` はProcedure.Textのfield名誤りによるcompile failure、`7b` は大文字小文字を区別しないfilesystemでの不適切な存在判定。DirEntry.Nameの厳密比較に直した `7c` が有効RED。途中のcompile failureはREDに数えない。
- 既存schema3・旧configフィールド・旧frontmatter・大文字tree期待は INVALID_FIXTURE として新契約へ追従した。Unit/成果commit/独立review/保存失敗のassertを削除していない。
- 配布のRule入口entry.mdと本文rule.mdはともにtype Rule。入口と全文読込を保持し、段階selectorに本文の既存title条件を明示した。親了承済みの通常詳細で、解決pathのrule.md一致も要求する。
- 明示TDD文書とtest proofを同じ検査版でacceptedへ保持する。integration時のproofは通常selector入力一覧との一致ではなく、accepted hash照合で検査する。共有current入力を一括固定しない。

## 検証境界と残す条件

親final用の実CLI四段階一周は `go test -tags=integration -count=1 -v ./src/cmd/aidlc -run '^TestIntentDocumentsJourney$'`。共通fixtureで実go testのRED/GREEN、文書一覧CLI、Sensor/review、全段階遷移を行う。既存integration fixtureも文書一覧API/小文字pathへ静的に追従したが、このloopではintegrationのcompile/実行を行っていない。live/full-project/race/vet/crossbuildも実行していない。最終検証の証拠は親PRへ記録する。

workspaceは追加許可の既存3fileのpath期待のみを変更した。通常testは `go test -count=1 ./src/internal/workspace -run '^(TestCreateSpaceOKF|TestFlowSpaceADR)$'`（exit0、`/tmp/stage-documents-workspace.log`）。integration tagのSpace tree試験は静的追従のみで親finalへ残す。

macOSで `src/core/minimal/knowledge/ADR/index.md` を filesystem上の `adr/index.md` へ二段階renameした。Git core.ignorecaseのため親のstage時に旧index entryを除き新小文字pathをaddし、case-only renameを確認する必要がある。実装担当はGit index/commitを操作していない。

## 作業単位末尾

`/tmp/stage-documents-final-1.log`〜`7.log` は上表の7 exact targetedを順に実行し、すべて数値exit0。`8.log` は許可されたaffected 7 packageの通常test、`9.log` はworkspace通常2件、`10.log` はgit diff --checkで、すべてexit0。変更Goは末尾でgofmtを適用した。0件test・skipをGREENとしていない。

追加の厳密型回帰は `TestDocumentDeclaration/numeric_type` と `numeric_tag`。非通常fileの停止防止は `TestDocumentSelectorNonRegular`。開始/終了の必須前提は `TestSelectedDocumentsRequiredInputs` / `RequiredTypes`、共有版とTDD proofの境界は `TestSelectedDocumentsIntegrationDoesNotFreezeMaterials` で検査した。

変更pathとSHA-256は `/tmp/stage-documents-files.sha256`、作業単位diffは `/tmp/stage-documents-final.diff` に保存する。これらは親の末尾確認用の一時証拠で、永続的な最終検証は親PRの対象HEADに結び付ける。WORK_UNIT_READYで実装編集を停止する。

## 独立レビュー修復（explicit-stage-documents-review-repair）

開始HEAD `a3d378d65f70acc86fdb60061bd7ad33e76e849d`、loop、Issue147の同一承認範囲。3findingを指定順で修復した。

1. ADR登録時の存在確認がReadFileでFIFOを開きSpace lockを保持した。`TestDocumentRegistrationNonRegular` で1秒以内に戻らない有効REDを確認し、内容を開かずos.Root.Lstatで各祖先と末尾の種類を検査する形へ変更した。FIFO/ディレクトリ/symlink拒否、revision保持、拒否後の同expect再登録と未存在output許可を確認。REDログ `/tmp/stage-documents-repair-1-red.log`、GREENは同`1-green.log`。初回shellのexit表示にはzshの予約変数statusを誤用したが、go testのassertion FAILログは保存されており、GREEN以降は専用変数で数値exitを確認した。
2. fresh Intentの文書一覧がnull配列で、取得JSONをそのまま登録できなかった。`TestIntentDocumentsRoundtrip` のRED(exit1)→GREEN(exit0)。読取り出力だけを空配列へ正規化し、永続stateの追加変更を避けた。ログは同prefixの`2-red.log`/`2-green.log`。
3. memory create/update helpの型例を`Design / adr / Rule`へ訂正。`TestIntentDocumentsLowercaseHelp` のRED(exit1)→GREEN(exit0)。ログは同prefixの`3-red.log`/`3-green.log`。

末尾は次の指定targetedだけを実行し、gofmtとdiff checkを行った。結果は `/tmp/stage-documents-repair-final-{1..5}.log` に数値exit付きで保存。全package/affected全体/race/vet/integration/buildは実行していない。旧大文字ディレクトリのGit rename、元worktree、人間承認実装は触っていない。

- `go test -count=1 ./src/internal/flow -run '^TestDocumentRegistrationNonRegular'`
- `go test -count=1 ./src/internal/minimal -run '^TestIntentDocumentsRoundtrip'`
- `go test -count=1 ./src/internal/cli -run '^TestIntentDocumentsLowercaseHelp'`
- `go test -count=1 ./src/internal/flow ./src/internal/minimal ./src/internal/cli -run '^TestIntentDocuments'`
- `git diff --check`

上記末尾5件はすべてexit0。修復ファイルhash一覧は `/tmp/stage-documents-repair-files.sha256`。WORK_UNIT_READYで編集を停止し、最終検証は親PRで対象HEADへ結び付ける。
