# configure helpの設定JSON例を実案件として改善する

状態: Accepted。ユーザーは「intent configure --helpに、Unitなし／ありの有効な設定JSON例を追加する改善」を
実案件として4段階で進める案に「この改善を実案件として進める」と回答した。直接承認であり旧ロードマップは流用しない。

## 現状と目的

基準mainは22a666c。現在のhelpはconfigのfield名を示すが、Unit内の型・初期値や完全なJSON例がない。
利用者がhelpから設定草稿を作れるよう、既存schemaを説明する2つの完全な例とfieldの型を加える。
例は実値へ置換する箇所を明記し、コピーだけでどのprojectでも検証済みになるとは説明しない。

## 変更と受入

- src/internal/cli/help.go: Unitなし／ありのconfig JSON例。配列、Unit id/bolt/base_commit/depends_on/scope/tests、
  status pending、空のresult_commit/integrated_commit、現在HEADの置換、既存進捗保全を説明する。
- src/internal/cli/configure_help_test.go: helpから例を抽出し、正しいJSONと必要説明を検査する。
- src/cmd/aidlc/configure_help_integration_test.go: 実Git HEADと成果物を用意し、help例のplaceholderを置換。
  公開configureが受理しplanning Sensorが受理することをUnitなし／ありの両方で確認する。
- 必要ならsrc/harness/codex/minimal/WORKFLOW.mdへconfigure helpの短い参照。
- 承認・TDD証拠・4段階実施結果をdocs/ramと索引へ記録する。

新API・schema・state遷移・外部module追加なし。Go単一バイナリを維持する。
helpの例は既存契約の説明なので本家AI-DLCのSpace・配布に新しい意図的差分を加えない。
今回の小さな変更は直接実装としUnitへ分割しない。Unitありの例自体は回帰testで検査する。

## 実案件の進め方

実際のai-dd worktreeへ製品をfresh配置し、一つのIntentで目的整理・計画・TDD・統合検証を進める。
親AIが公開CLIでstateを更新し、別rootの独立reviewerの実報告を受理してから各境界を進める。
Knowledgeには現行入力と使い方をCLIで保存。アーキテクチャ変更がないのでADR不要理由をstateへ記録する。
製品データと配置された絶対path設定はこの実案件worktree内に保持し、製品PRへ混ぜない。
この実施は本会話のAIとsubagentによる実CLI利用。新しいCodex CLI会話でhook自動発火まで実測したliveとは区別する。

## TDD・所有・検証

単独実装担当のwork_unit_id=configure-help-pilot、verification_mode=loop。
1. 説明・例の欠落を確認するtestを先に追加し、実行可能なREDからhelpを実装する。
   `go test -count=1 ./src/internal/cli -run '^TestConfigureHelp'`
2. 公開CLIで例を使いplanning Sensorまで確認する。
   `go test -tags=integration -count=1 ./src/cmd/aidlc -run '^TestConfigureHelpExamples$'`
既に成立する期待はALREADY_GREENと記録し、人工的なREDを作らない。
末尾に両targetedとgofmt・diff check、親boundary・独立reviewを行う。

安定後のfinalは全test、race/shuffle、vet、tidy-diff、format/diff、全integration、6構成build、native help。
長時間の既存liveは繰り返さず、今回の実案件の4段階と実CLIの例検証を証拠とする。
最終の製品レビュー対象を変更後の古いpassで進めない。完了state自身はコード検証対象版と区別する。
Issue分類はCLIの公開help挙動改善として機能開発。独立review・final・対象CI成功後に通常merge commit。
ユーザーの未commit変更、参照資料、他worktreeを保全する。Gitで戻せる変更のみ。
