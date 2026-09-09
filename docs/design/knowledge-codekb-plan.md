# 現行知識の保存先をcodekbへ統一する実装計画

## 背景・許可・利用結果

現在の配布はSpaceのOKF検索ルートknowledge内に、さらにknowledgeという現行知識フォルダを作成する。
ユーザーが`knowledge/knowledge`を`knowledge/codekb`へ変更するよう直接依頼したため、保存先と関連する参照を揃える。
根拠は[直接承認RAM](../ram/decisions/2026-09-09-knowledge-codekb-approved.md)。旧33 Stageの包括承認を流用しない。

利用者は`aidlc memory create codekb/<名前>`で現行仕様を保存し、Space内の通常OKF検索で取得する。
現状解析はcodekb/current-analysis.md、構成図はcodekb/architecture.md、機能仕様はcodekb/<機能名>.mdとなる。

## 範囲と境界

- 新規配布とSpace作成で`aidlc/spaces/<space>/knowledge/codekb/index.md`を配置し、Bundle索引を更新する。
- Sensorの既定解析/構成図pathと統合検証の機能Knowledge判定をcodekbへ揃える。前段合格版やmetadata条件は維持する。
- 開始前の文書修復hook、stage MD outputs、CLI helpと例、現在の開発ガイドを更新する。
- 外側のknowledgeはOKF検索rootのまま。design/adr/rules/log、Intent state、4段階遷移、OKF typeは変更しない。
- memory CLIは一般のConcept IDを扱うため、旧名のフォルダを含む任意Conceptの読取や作成を一律禁止する機能は追加しない。製品の既定配置・必須成果物・案内を変更する。
- 新規Intent/新規配置向け。利用者の既存ファイルを削除・移動・上書きしない。保存済みstateの自動移行、旧経路互換、二重配布は作らない。配布sourceのフォルダrenameは本依頼に含む。
- Goシングルバイナリを維持し、外部Go moduleやtoolを追加しない。

固定OKF v0.2のBundle配下のConcept pathを変更する製品上の配置変更であり、OKF formatや本家固定AI-DLC2.6.123のworkflow動作を追加変更しない。過去のsnapshot・RAMは履歴として保持する。

## 単独writerの所有範囲・TDD

work_unit_id: knowledge-codekb。1 Issue/PRの全変更を単独go_tdd_implementerが実装する。親はIssue、計画、独立review、final、PRを管理する。

1. 新規配置: src/core/minimal/knowledge/knowledge/index.mdをcodekb/index.mdへrenameし、Bundle index.mdを更新。src/internal/installの新TestCodeKBDistribution、必要なworkspace/space_create関連fixtureを追加・修正する。exact: `go test -count=1 ./src/internal/install -run '^TestCodeKBDistribution'`。旧配布folder不在、別Space作成でもcodekbを配布、索引リンクが実在することを確認。
2. Sensor/修復hook: src/internal/flow/boundary_documents.go・boundary_end.go、src/internal/minimal/hook.go、新しいcodekb_test.go等と関連flow/minimal fixtures。exact: `go test -count=1 ./src/internal/flow ./src/internal/minimal -run '^TestCodeKB'`。codekbの共有解析/図と機能Knowledgeが検査を通り、統合の旧knowledgeフォルダが新しい必須配置を代用しないこと、codekbの正規修復が開始前に許可されることを検証。
3. 手順/CLI/結合fixture: src/core/workflow/stages/*.md、src/harness/codex/minimal/WORKFLOW.md、src/internal/cli/help.goと関連test、docs/development.md、src配下の現行memory Concept例とtest fixturesを必要範囲で追従する。exact: `go test -count=1 ./src/internal/cli ./src/internal/install -run '^TestCodeKB'`。procedure出力path・helpと配布手順がcodekbで一致することを検証。既存実CLI journeyとworkspace配布snapshotはassertionを新pathへ更新し実行はfinalへ集約する。

各sliceはテストを先に追加し、意図したREDから最小GREENへ進む。すでに成立したものはALREADY_GREENと記録する。新API scaffold不要。単なるfixtureの旧既定名追従は意味を変えず一括更新できるが、OKF仕様自体や旧snapshot内のknowledge一般名は機械置換しない。

作業単位末尾に上記targeted3コマンドと`go test -count=1 ./src/internal/flow ./src/internal/minimal ./src/internal/cli ./src/internal/install ./src/internal/workspace ./src/internal/okfmemory ./src/internal/workflow`を実行し、変更Goのgofmtとgit diff --checkを行う。loopで全体/race/vet/build/E2Eを実行しない。RAMへ実装証拠を記録し索引を更新する。

## 受入とfinal

新規installとSpace createの配置、正規memory create/search/show、開始修復、解析/図・機能文書Sensor、4段階の実CLI完走でcodekbを使用できる。旧保存先を新製品の手順・helpが案内しない。design/adr/rules/logを誤って変更しない。既存利用者ファイルは保持する。

固定差分の独立レビュー後に親がread-only finalを1回実行する。`go test -shuffle=on ./...`、`go test -race -shuffle=on ./...`、`go vet ./...`、`go mod tidy -diff`、`gofmt -l src`、`git diff --check`、`go test -tags=integration -count=1 ./...`、darwin/linux/windowsのamd64/arm64 build、native help/versionを検証する。現在HEADの全GitHub checks成功後、通常merge commit方式でマージしIssue close/main反映を確認する。

リスクは分散したpath参照の取り残し。限定検索と新配置・Sensor・hookの回帰テストで検査する。ロールバック時も既存文書を自動削除・変換せず旧版へ戻して利用を止める。中断中の人間承認branchと利用者資材には触れない。
