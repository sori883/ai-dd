# ステージのOKF入力検索と出力宣言

## 目的と許可

現在の段階Markdownはconfig.adr.refs等を参照するため、読む文書と保存先が分かりにくい。入力はOKFのfrontmatter（文書先頭のmetadata）で検索し、出力は具体的な保存先と期待metadataで定義する。ユーザーはこの方式、Intentごとの可変出力一覧、必要なmetadataの追加を直接承認した。ADRのフォルダとtypeは小文字にする明示指定も含む。根拠はdocs/ram/decisions/2026-09-09-stage-okf-selectors-approved.mdと2026-09-09-lowercase-adr-request.md。旧33段階ロードマップの許可は流用しない。

mainの8b05f9cc79185459e43cf6a05c7da18aeeb605f3を起点に、別worktreeで1 Issue/PRとして実装する。人間承認Issue146は中断したまま取り込まない。Go単体バイナリと既存依存を維持し、新module・外部tool・権限は追加しない。

## 利用者の操作

`aidlc intent documents ID --space SPACE`で登録一覧を読む。
`aidlc intent documents ID --space SPACE --expect REV --file documents.json`で一覧全体を更新する。
expectは現在stateのrevisionで、競合は明示拒否する。登録は文書自体の作成・変更を行わない。本文は従来のmemory create/updateで保存する。

```json
{
  "inputs": [{"stage":"planning","path":"aidlc/spaces/default/knowledge/adr/storage.md","metadata":{"type":"adr","title":"保存方式","description":"採用済みの方式と理由"}}],
  "outputs": [{"stage":"integration","path":"aidlc/spaces/default/knowledge/knowledge/order.md","metadata":{"type":"Knowledge","title":"注文機能","description":"注文の現行仕様と利用手順"}}]
}
```

pathは選択SpaceのKnowledge内のrepository相対Markdownパス。可変出力は複数登録可で、機能名や判断名を維持する。metadataはtype/title/descriptionの非空文字列が必須。statusとtags、intent_idは任意で、明示時は検査する。statusは既存draft/stable/deprecated、tagsは既存文字列配列。日時・生成者は宣言で偽装せずmemory CLIが生成する。指定しないstatusをstable必須とはしない。

RequirementsとImplementationPlanのintent_idは今回IDをCLIが補い、異なる指定は拒否する。新規ADR出力は登録時点で文書が未存在なら今回IDを補って永続化し、後からファイル存在だけで判断し直さない。既存共有Knowledge・採用済みADRにIntentIDの空更新を要求しない。明示したintent_idは完全一致で検査する。

## 段階定義

inputsはmatch、count、versionで指定する。

```yaml
inputs:
  - match:
      type: Requirements
      intent_id: "${intent_id}"
    count: one
    version: accepted
    accepted_at: discovery
outputs:
  - role: implementation_plan
    path: "${knowledge_root}/design/${intent_id}/implementation-plan.md"
    metadata:
      type: ImplementationPlan
      intent_id: "${intent_id}"
```

共通定義の固定outputsはpathとmetadataを持つ。title/descriptionは特定文言へ固定せず、保存文書では非空を要求する。Intentに登録した可変文書は`declared: intent_documents`で現在段階のinputsまたはoutputs一覧を展開する。これは一覧展開の明示指示でありconfig.*をたどらせる指定ではない。intent procedureは未作成出力を含む具体的pathと期待metadata、入力検索条件・件数・解決候補・版を表示する。必須入力欠落でも手順取得は可能とし、診断を返す。開始合否はSensorが決める。

matchはtype（必須）、intent_id、title、description、status、tagsを許可し文字列完全一致AND、tagsは指定配列と順序によらない集合一致を用いる。曖昧な全文検索とは別の厳密selectorとして実装する。countはone=1件、optional=0～1件、many=1件以上。未知field/型/値は拒否する。現在Spaceの実ファイルだけを安全に走査し、別Space・symlink・破損文書を成功に数えない。結果順はpathで安定させる。

Ruleはtype Ruleを必須1件として既存rules/rule.mdとの一致も確認し、必須Ruleの読込契約を維持する。CurrentAnalysis/Architectureは共有typeで0～1件、資材があるdiscovery終了とintegration終了では各1件を要求する。要件と計画はtype＋今回intent_idで必須1件、後段では前段の合格版を要求する。現在IntentのADRと採用済み過去ADRを区別し、過去ADRは登録した具体pathで検査する。全SpaceのADRを一律に採用しない。

outputsは必ず宣言pathの現物をmetadata・既存必須節と照合し、別pathの検索ヒットで代用しない。integrationで最低1件の機能Knowledge出力を要求する。ADR requiredなら現在/前段の宣言から必要ADRを検査し、不要なら既存reasonを要求する。新規ADRはknowledge/adr/、type adr。他文書typeやArtifactの役割名は変更しない。

## 進捗・保存・検査

state schemaはmainの3から4にし、config.document_inputs/document_outputsに一覧を保存する。旧feature_knowledgeとadr.refsは除去し旧fieldを拒否する。configureは保存済みの新一覧を保持し、専用CLI以外から変更できないようにする。schema3の自動移行や実ファイルの改名は行わず保持したまま明示エラーにする。中断中の人間承認branchもschema4を使っているため、その再開時は衝突しないschemaへ調整する必要がある。

一覧更新はSpace lockとrevision照合、既存原子的state保存を使う。途中失敗は旧stateを保ち、同じexpectで再試行可能。旧Sensor/reviewを失効させ、開始入力の指定変更はentryを無効化する。前段合格済みの宣言変更は該当段階へのreopenを要求する。現在・将来の出力追加だけでentryを不必要に消さない。

開始時に解決した入力path＋hashを保存する。同一段階中に再検索で別pathへ黙って交換せず、追加一致による曖昧化・消失も検出する。accepted文書は同じ合格済みpath＋hashと照合し、変更したら前段へreopenする。共有current文書は正当な更新を許し終了時の現行版をreview対象へ含める。開始時に不存在だったoptional共有文書はその段階で作成可能。outputsとinputsが重なる共有文書も更新を許す。全操作auditや本文snapshotは追加しない。

startState/endDocuments/procedure.references等の固定path・config参照の二重判定を同じ解決処理へ寄せる。文書型・必須節・Mermaid・Intent対応・開始/終了版・定義hash固定・Unit依存/担当/commit・テスト実測・独立reviewの検査は維持する。memory修復用hookも解決した入力/宣言した出力へ対応し、未開始時の必要文書作成が行き止まりにならないようにする。現在の4段階・戻り理由のwork-logを維持する。

## 所有範囲・実装順序

work_unit_id=explicit-stage-documents、verification_mode=loop、単独Go writer。所有範囲はsrc/internal/{okfmemory,workflow,flow,cli,minimal,install}と対応test、src/core/{workflow,minimal}、src/harness/codex/minimalの手順、src/cmd/aidlcのCLI/integration fixture、docs/development.md、実装証拠RAMと索引。親は計画/承認/GitHubを管理する。元worktree・ユーザー資材には変更しない。lowercase配布に直接依存するsrc/internal/workspaceの既存3testについては期待pathのADR→adr更新だけを所有範囲へ追加する。製品workspace動作や他のassertionは変更せず、該当test名のexact -runで確認する。

|順|受入・TDD slice|exact targeted command|
|---|---|---|
|1|metadata完全一致、件数、不正文書、Space境界|`go test -count=1 ./src/internal/okfmemory -run '^TestDocumentSelector'`|
|2|match/path+metadata/declared定義、旧refs拒否|`go test -count=1 ./src/internal/workflow -run '^TestDocumentDeclaration'`|
|3|文書一覧登録、schema、CAS/失敗、configure保全、前段変更拒否|`go test -count=1 ./src/internal/flow -run '^TestIntentDocuments'`|
|4|入力一意性/版/検索すり替え拒否、共有更新、出力pathとmetadata|`go test -count=1 ./src/internal/flow -run '^TestSelectedDocuments'`|
|5|adr小文字・複数文書・既存必須Sensor維持|`go test -count=1 ./src/internal/flow -run '^TestLowercaseADR'`|
|6|CLI/help/procedure解決表示・hook修復操作|`go test -count=1 ./src/internal/cli ./src/internal/minimal -run '^TestIntentDocuments'`|
|7|fresh配布の段階定義・help・型/パス|`go test -count=1 ./src/internal/install -run '^TestDocumentDistribution'`|

各sliceはtest先行でrunnable REDを確認してから最小GREENへ進む。新APIの型/signature/空返値だけのcompile scaffoldは許可する。既存で成立する追加coverageはALREADY_GREENを実測し人工REDを作らない。明らかな旧schema・旧配布fixture不一致は受入を弱めず更新し、無関係な失敗をREDと数えない。

末尾で7指定commandとaffected package（okfmemory/workflow/flow/cli/minimal/install/cmd）の通常testを確認し、変更Goのgofmtとdiff checkを行う。親は末尾で7commandを一度再実行し、固定base/headの独立reviewへ渡す。integration/live/race/vet/全体test/crossbuildはloopやreviewで行わない。

## 最終検証と配布

blocking finding修復後、親がread-only finalで `go test -shuffle=on ./...`、`go test -race -shuffle=on ./...`、`go vet ./...`、`go mod tidy -diff`、`gofmt -l src`、`git diff --check`、`go test -tags=integration -count=1 ./...`、darwin/linux/windows×amd64/arm64のbuildを実行する。実CLI journeyで検索入力→文書宣言→memory保存→4段階のSensor/review/前進を検証し、native helpとprocedureの実ファイル一覧を確認する。実AIの自然言語判断品質を非live検証の成功から主張しない。

配布原稿とfresh installを更新する。旧環境の自動更新は追加しない。独立review/final/対象headのGitHub全checks成功後、通常mergeしmainとIssue closeを確認する。

## 本家との関係

OKF v0.2（固定commit ad30107c31c06aec8a7d5636e0d1058118604e6f）の任意type・標準metadataを利用する。検索条件と出力宣言は製品独自の4段階契約で、OKF規格の追加要求ではない。固定AI-DLC2.6.123の33段階へ戻す変更ではなく、既に承認された最小4段階を読みやすくする意図的変更。利用者は設定名の間接参照に代わり、metadata条件と具体的出力一覧を利用する。現在の版固定・新規配置のみ・旧データ非移行という境界は維持する。
