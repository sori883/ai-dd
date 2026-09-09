# ステージ実行計画を提案する専用エージェント

状態: Accepted。専用担当の役割・対象・検証を提示した後、ユーザーが「はい、そのエージェントを作ってもらっていいですか。お願いします」と実装を直接承認した。

## 背景と目的

PR154のmainでは、目的整理・深掘りの中でメインAIがステージの採否と順序を提案する。ユーザーは、ステージを決める専用のエージェントが欲しいと依頼した。目的整理と実行計画作成を同じdiscovery内の順序付き手順にする方向は直前の会話で確認している。

専用の読み取り担当aidlc-stage-plannerを追加する案とする。製品のエージェントであり、本リポジトリ開発用project_plannerとは別物。

## 役割と入出力

要件整理担当が目的・要件・受入条件を整理し、調査担当が既存資材・OKF・公式資料の根拠を返した後、メインAIがaidlc-stage-plannerを起動する。

入力はIntent/Space、現在のstateと実行計画、6段階カタログと手順、必須Rule、要件、調査結果、制約、利用可能な成果物。入力が不足する場合は根拠と確認質問をメインAIへ返す。

出力は、実行ステージと順序、各選択の理由、省略理由、期待成果物、不足情報、および既存intent plan CLIへ渡せるPLAN.json案。初期化とdiscoveryを保持し、構成分析・planning・tdd・integrationの採否を明示する。既存step IDと完了prefixを保持し、新規step IDはCLIに採番させる。途中変更にも同担当を利用し、reopenが必要な場合は対象実行回と後続への影響を明示する。

ソースコードの実装手順やUnit詳細を作るplanningステージとは区別し、実行するステージを選ぶ責任を持つ。広い追加調査はresearcherへ戻し、独立レビューも兼任しない。承認、state/Knowledge保存、サブエージェント起動はメインAIが行う。新担当はread-onlyで案を返し、利用者の回答や承認を作らない。

最終決定者はユーザー。メインAIが案を説明して承認を受け、既存plan/plan-approvalで記録する。計画承認と成果承認、変更ごとの承認、6ステージ、既存state schemaは維持する。担当呼出しをdiscoveryと計画変更時の手順へ記述し、CLIが自動起動する機構や別の起動監査は追加しない。

## 実装対象

- src/harness/codex/minimal/agents/aidlc-stage-planner.tomlを追加。fresh installで.codex/agents/へ既存方式で配置。
- src/core/workflow/stages/discovery.mdへexecution_planning役割と呼出し順を追加。
- src/internal/workflow/definition.goの許可する役割/agent対応、および関連testを更新。
- src/harness/codex/minimal/SKILL.md、WORKFLOW.mdとdocs/development.mdへ5担当の分担、メインAIによる依頼・結果回収・ユーザー承認を反映。
- src/internal/install/product_agents_test.goと関連workflow/install/cmd testで配布・手順・正規procedure表示を確認。
- 本RAMと索引、実装証拠を更新。

定義hashが変わるため、従来どおり新しい配布・新Intentで利用する。旧定義に結び付いたIntentを自動移行せず、既設ファイルの上書きは行わない。Go単一binaryと標準ライブラリを維持し、外部module/toolは追加しない。

## 受入と検証

1. discoveryのprocedureに新担当と起動する順序が表示される。
2. 新agentに要件・調査結果を渡すと、6段階の契約に従った選択/省略理由とPLAN案を返すよう指示される。
3. agentは読み取り専用で、メインAIの保存・会話承認・独立review境界を維持する。
4. fresh installに5担当が配置され、未知のrole/agent対応の拒否も維持する。

承認後に1 Issue、単独writer、1 work unitとして進める。TDD順は定義parser→配布/手順→procedure表示。targeted commandsは `go test -count=1 ./src/internal/workflow -run '^TestStagePlanner'`、`go test -count=1 ./src/internal/install -run '^TestStagePlanner'`、`go test -count=1 ./src/internal/flow -run '^TestStagePlanner'`。既存fixtureは必要な範囲で追従する。

独立review後の固定HEADで全体test/race/vet、format/tidy/diff、配布journey、6構成buildを確認する。既存の固定Codex環境で新named agentの起動・案の返却・共有ファイル不変更を限定実測し、配置だけで起動確認済みとはしない。GitHub checks成功後に通常のPR/merge手順でmain反映を確認する。

## 実装許可の境界

専用担当の追加は今回の新しい要望であり、既存4担当やPR154の実装許可を流用しない。本案の役割・対象・検証への直接承認を得たため、この範囲のコード、設定、Issue、PR変更と、品質gate後のmergeを進める。
