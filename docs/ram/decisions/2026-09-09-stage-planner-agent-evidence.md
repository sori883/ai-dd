# Issue #155 ステージ計画担当の実装証拠

状態: loop完了。work_unit_idはissue155-stage-planner。実装許可は[直接承認済み計画](2026-09-09-stage-planner-agent-request.md)、Issueは#155。単独writerでcodex/stage-planner-agentへ実装した。開始HEADはb925bf62eda219edfcf7592bd967e7892a48d0c5。

## 順序付きTDD

すべてrunnableな期待値失敗を先に観測してから対応実装を追加した。compile失敗、人工RED、ALREADY_GREENはない。

| 項目とexact command | RED実測 | GREEN実測 | 境界再確認 |
| --- | --- | --- | --- |
| parser: `go test -count=1 ./src/internal/workflow -run '^TestStagePlanner'` | exit 1、chunk 39c619。正規execution_planning/aidlc-stage-plannerがinvalid agent role | exit 0、60c00a | exit 0、5b42e6、0.446s |
| 配布: `go test -count=1 ./src/internal/install -run '^TestStagePlanner'` | exit 1、9c1c93。fresh installに新agentファイルなし | exit 0、9deb4e | exit 0、810857、0.365s |
| procedure: `go test -count=1 ./src/internal/flow -run '^TestStagePlanner'` | exit 1、526820。担当が旧3件で新担当と順序付き本文がない | exit 0、806cd2 | exit 0、df4a44、0.397s |

各chunkは実行toolの出力識別子。新規testは正規roleの受理と未知role/誤agent拒否、fresh installのread-only指示・共通入口、procedureの担当順と承認境界を確認する。既存配布一致回帰 `go test -count=1 ./src/internal/install -run '^TestProductAgent'` もexit 0（5db6df、0.199s）。

変更Goファイルへgofmtを適用しexit 0（4c01c1）。`git diff --check` はexit 0（109133）。

## 実装結果と境界

埋込みagents配下にaidlc-stage-planner.tomlを追加し、既存配布方式で配置する。model/effortを固定せずread-onlyとした。要件整理・調査後と途中変更時にメインAIが呼び、採否・順序・理由・省略理由・期待成果・不足情報とCLI形式のPLAN案を返す。共有保存・ユーザー回答の取得・plan承認はメインAIが担当する。planningの実装手順やUnit詳細、researcherの広い調査、reviewerの独立レビューと分担する。

discoveryの定義と共通入口、開発文書を更新した。schemaと6段階は変更せず、旧Intent移行・既設上書き・外部module追加は行っていない。原本checkoutは編集していない。

## 親finalへの引継ぎ

loopでは全体test/race/vet/build/E2E/liveを実行していない。既存4担当smokeを適応した親の `/tmp/ai-dd-stage-planner-final.py` と `/tmp/ai-dd-stage-planner-live.py` を利用する。fresh install後にexact named agentを起動し、SubagentStart/Stop、全文返答の6段階/PLAN適合、共有プロジェクト不変更を照合する。配置testの成功を実機起動や成果品質の保証としない。定義hash変更に伴う新配布・新Intent利用は既存契約どおり。

## 実機観測後の文書指示修正

work_unit_id: issue155-doc-output-repair、verification_mode: loop。開始HEADはdf51a82373d05195ec691d3371b1bce8d0f9dacf。親の実機検証では新agent起動・返答・共有ファイル不変更が成功したが、期待成果欄にTDDのテスト、help変更、commitが文書成果と混在した。既決のoutputsは文書だけで、ない場合は「なし」とする境界に合わせ、agent原稿・discovery・共通入口・開発文書を明確化した。プログラム、テストコード、commitは文書outputsへ列挙せず、検証証拠の必要性は別に説明する。schema/API変更はない。文書指示修正のため人工REDは作らない。

修正後の確認:

- `go test -count=1 ./src/internal/workflow -run '^TestStagePlanner'`: exit 0、b51eaf。
- `go test -count=1 ./src/internal/install -run '^TestStagePlanner'`: exit 0、fcd3c1。
- `go test -count=1 ./src/internal/flow -run '^TestStagePlanner'`: exit 0、f8b813。
- `git diff --check`: exit 0、3dd8f3。Goファイル変更なしのためgofmt適用は不要。全体検証は実行していない。

## 文書outputs制約の配布回帰

work_unit_id: issue155-output-regression、verification_mode: loop。開始HEADはa9747bec4510224dc620123b428ed449c8d8d782。再reviewのP2を受け、TestStagePlannerDistributionで「期待する文書（なければなし）」「プログラム・テストコード・commitを文書outputsへ列挙しない」「検証証拠の必要性は文書outputsとは別に説明する」を配置済みagent本文から検査する。原稿は既に正しいためALREADY_GREENであり、人工REDは作っていない。

`go test -count=1 ./src/internal/install -run '^TestStagePlanner'` はexit 0（935239、0.407s）。当該testへのgofmtとgit diff --checkもexit 0。同一work unitでは製品原稿を変更せず、全体検証も実行していない。
