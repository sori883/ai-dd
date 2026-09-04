# 開発プロジェクトRAM

## 目的

このディレクトリは、GoでAI-DLCを再実装する開発プロジェクトの意思決定、調査結果、
前提、未解決事項を継続的に記録する。

AI-DLCが利用プロジェクト内で管理する`aidlc/spaces/<space>/knowledge/`やDocumentKBとは
役割が異なる。ここにはAI-DLCという成果物を開発する側の知識だけを置き、AI-DLCを使って
開発される個別成果物の知識は置かない。

## 分類

- `decisions/`: 承認済みまたは再検討中の設計・運用上の意思決定
- `research/`: 参照実装、仕様、技術選択肢の調査結果

各記録には、日付、状態、背景、結論、影響、未解決事項、根拠を必要な範囲で含める。
決定を変更する場合は過去の記録を消さず、新しい記録から置換対象を参照する。

一時的な作業メモ、秘密情報、利用プロジェクト固有のAI-DLC knowledgeは保存しない。

## エージェント運用

1. 計画、設計、調査、実装を始める前に、この索引と関連記録を確認する。
2. ユーザーへ既出事項を質問する前に、関連する意思決定がないか確認する。
3. 今後の作業に影響するユーザーの回答、承認、方針変更は、原則として同じタスク内で記録する。
4. 記録を追加または更新したら、下記の索引も更新する。
5. 既存決定を変更する場合は新しい記録を作り、置換した記録への参照を残す。

この運用の必須ルールは、リポジトリルートの`AGENTS.md`に定める。

## 索引

| 種別 | 記録 | 状態 |
| --- | --- | --- |
| 意思決定 | [初期実装の境界](decisions/2026-08-29-initial-implementation-boundaries.md) | Accepted |
| 意思決定 | [プロジェクトRAMの記録運用](decisions/2026-08-29-project-ram-policy.md) | Accepted |
| 意思決定 | [Project root解決の初期契約](decisions/2026-08-30-project-root-resolution.md) | Accepted |
| 意思決定 | [Local配布E2E sandboxの運用](decisions/2026-08-30-local-distribution-e2e-sandbox.md) | Accepted |
| 意思決定 | [内部workspace機能を先行し、statusを後段で実装する](decisions/2026-08-31-internal-workspace-before-status.md) | Accepted（実装順序） |
| 意思決定 | [共通space読み取りの初期契約](decisions/2026-08-31-space-reading-contract.md) | Accepted |
| 意思決定 | [Intent読み取りの実装計画](decisions/2026-08-31-intent-reading-plan.md) | Accepted |
| 意思決定 | [Workspace読み取り接続の実装計画](decisions/2026-08-31-workspace-reading-composition-plan.md) | Accepted |
| 意思決定 | [Space作成をCLIから使えるようにする実装計画](decisions/2026-08-31-space-creation-plan.md) | Accepted（Issue #19、strict flag値・SIGPIPE修正・最終配布E2Eまで記録） |
| 意思決定 | [Space一覧をCLIへ接続する実装計画](decisions/2026-08-31-space-list-plan.md) | Accepted（Issue #21、引数境界の具体化、TDD・独立レビュー・53起動の配布E2Eを記録） |
| 意思決定 | [Space切替を共有カーソルへ接続する実装計画](decisions/2026-09-01-space-switch-plan.md) | Accepted（Issue #23、17項目TDD・独立レビュー・76起動の配布E2Eを記録） |
| 意思決定 | [Intent一覧をCLIへ接続する実装計画](decisions/2026-09-01-intent-list-plan.md) | Accepted（Issue #25、16項目TDD・P1修正後の独立レビュー・45起動の配布E2Eを記録） |
| 意思決定 | [Intent切替を共有カーソルへ接続する実装計画](decisions/2026-09-01-intent-switch-plan.md) | Accepted（Issue #29、13項目TDD・P2/P3修正後の独立review・32起動の配布E2Eを記録） |
| 意思決定 | [Intent作成の内部coreとworkspace lockの実装計画](decisions/2026-09-01-intent-create-core-plan.md) | Accepted（Issue #31、13項目TDD＋Go 1.27回帰修正・review・6構成cross compile、PR #32） |
| 意思決定 | [読み取り専用ワークスペース分析の実装計画](decisions/2026-09-02-workspace-detection-plan.md) | Accepted（Issue #33、7項目TDD・P1修正後の独立review・6構成cross compile） |
| 意思決定 | [Stage graph・scope routing内部APIの実装計画](decisions/2026-09-02-stage-routing-plan.md) | Accepted（Issue #35、TDD・P1/P2修正後review・6構成cross compile） |
| 意思決定 | [Scope metadata read-only APIの実装計画](decisions/2026-09-02-scope-metadata-plan.md) | Accepted（Issue #37、7項目RED/GREEN＋block-first・ECMAScript trim・raw改行とinner backtrackingを含むblock regex parity修正・Runner ownership guard・最終review指摘なし・6構成cross compile） |
| 意思決定 | [初期 aidlc-state.md builderの実装計画](decisions/2026-09-02-initial-state-builder-plan.md) | Accepted（Issue #43、5項目TDD・P1修正後の独立review完了） |
| 意思決定 | [初期state永続化writerの実装計画](decisions/2026-09-02-initial-state-writer-plan.md) | Accepted（Issue #45、独立review・final完了） |
| 意思決定 | [4層Memory source readerの実装計画](decisions/2026-09-02-memory-source-reader-plan.md) | Accepted（Issue #47、独立review・final完了、Go 1.26.8で全検証） |
| 意思決定 | [Memory bundle filterの実装計画](decisions/2026-09-02-memory-bundle-filter-plan.md) | Accepted（Issue #49、独立review完了、final検証結果はPRへ記録） |
| 意思決定 | [検証頻度をloop・review・finalへ分離する](decisions/2026-09-02-validation-cadence.md) | Accepted（Issue #39、修正中はtargeted、差分安定後に全検証を1回） |
| 意思決定 | [go_tdd_implementerをLuna / maxで運用する](decisions/2026-09-02-go-tdd-implementer-luna-max.md) | Accepted（Issue #41、実装担当のみLuna / maxへ固定） |
| 意思決定 | [サブエージェントhandoffのコンテキスト予算](decisions/2026-09-03-subagent-context-budget.md) | Accepted（Issue #59、全文継承を例外化し、調査担当はTerra / mediumを維持） |
| 意思決定 | [本家AI-DLCとの差分を自発的に提示する](decisions/2026-08-31-upstream-difference-reporting.md) | Superseded（下記の意図的な差分に限定する方針へ置換） |
| 意思決定 | [本家との差分提示を意図的な仕様・挙動の変更に限定する](decisions/2026-08-31-intentional-upstream-difference-reporting.md) | Accepted |
| 意思決定 | [GitHub Issue・PRを日本語の実装記録として運用する](decisions/2026-09-01-japanese-github-pr-workflow.md) | Partially Superseded（日本語・履歴確認は維持、自動マージ禁止は下記決定で置換） |
| 意思決定 | [ロードマップ単位の包括承認と自律マージを採用する](decisions/2026-09-03-milestone-authorization-and-autonomous-merge.md) | Accepted（Issue #67、現在のAI-DLC Go実装ロードマップを最初の包括承認枠とし、品質gate後に自律マージ） |
| 意思決定 | [GitHub Issueを主要な成果で分類する](decisions/2026-09-01-github-issue-classification-labels.md) | Accepted（`機能開発` / `ユーザーリクエスト`、全14 Issueへ適用） |
| 意思決定 | [計画・Issue・PRを自己完結した分かりやすい文章にする](decisions/2026-09-02-self-contained-development-artifacts.md) | Accepted（Issue #51、今後生成する成果物へ適用） |
| 意思決定 | [OKF v0.2参照基盤と初期統合境界](decisions/2026-09-03-okf-reference-boundaries.md) | Partially Superseded（Issue #53、Stage実行中固定はin-flight recompose方針で置換、その他のOKF境界は維持） |
| 意思決定 | [AI-DLC Go実装ロードマップ（概要）](decisions/2026-09-03-aidlc-implementation-roadmap.md) | Partially Superseded（全体順序は維持、Stage実行中固定とPRごとの承認待ちは後続決定で置換） |
| 意思決定 | [Stage catalog metadataの実装計画](decisions/2026-09-03-stage-catalog-metadata-plan.md) | Accepted（Issue #55、TDD・loop検証・独立review・final gateを記録） |
| 意思決定 | [Intent開始時Stage Plan builderの実装計画](decisions/2026-09-03-stage-plan-builder-plan.md) | Accepted（Issue #57、ユーザー明示承認済み） |
| 意思決定 | [StartIntent内部接続の実装計画](decisions/2026-09-03-start-intent-plan.md) | Accepted（Issue #61、ユーザー明示承認済み） |
| 意思決定 | [in-flight recompose方針](decisions/2026-09-03-inflight-recompose-policy.md) | Accepted（旧ロードマップ／Stage Planの実行中固定方針を置換対象として参照） |
| 意思決定 | [保存済み aidlc-state.md readerの実装計画](decisions/2026-09-03-state-reader-plan.md) | Accepted（Issue #63、ユーザー明示承認済み） |
| 意思決定 | [Current directive resolverの実装計画](decisions/2026-09-03-current-directive-resolver-plan.md) | Accepted（Issue #65、ユーザー明示承認済み） |
| 意思決定 | [Stage completion artifact presenceの実装計画](decisions/2026-09-03-stage-artifact-presence-plan.md) | Accepted（Issue #69、ロードマップ包括承認内） |
| 意思決定 | [薄いライフサイクルを内部APIで完走するマイルストーン](decisions/2026-09-03-thin-lifecycle-milestone.md) | Accepted（ユーザーが残り7 PRの内部walking skeletonを明示承認） |
| 意思決定 | [Stage完了可否のread-only判定計画](decisions/2026-09-03-stage-completion-decision-plan.md) | Accepted（Issue #71、薄いライフサイクルマイルストーン内） |
| 意思決定 | [byte-preserving state transition patcherの実装計画](decisions/2026-09-03-state-transition-patcher-plan.md) | Accepted（薄いライフサイクルマイルストーン内） |
| 意思決定 | [既存state atomic update writerの実装計画](decisions/2026-09-03-state-update-writer-plan.md) | Accepted（薄いライフサイクルマイルストーン内） |
| 意思決定 | [最小audit ledgerとrecord lockの実装計画](decisions/2026-09-03-audit-record-lock-plan.md) | Accepted（薄いライフサイクルマイルストーン内） |
| 意思決定 | [承認ゲート遷移と人間応答監査記録の接続計画](decisions/2026-09-04-approval-gate-receipt-plan.md) | Accepted（Issue #79、薄いライフサイクルPR5、reader所有の承認根拠・ECMAScript trim回帰修正を記録） |
| 意思決定 | [承認から次Stage・workflow完了までの接続計画](decisions/2026-09-04-approve-advance-plan.md) | Accepted（Issue #81、薄いライフサイクルPR6、二段階audit-first保存・実装記録） |
| 意思決定 | [内部Next・Reportとライフサイクル一周テストの計画](decisions/2026-09-04-next-report-lifecycle-plan.md) | Accepted（Issue #83、薄いライフサイクルPR7、内部入口・一周E2E・CI・Report対象拘束の回帰修正） |
| 意思決定 | [ルール・知識・工程定義を本文入りの検討用資産として配置する](decisions/2026-09-04-aidlc-content-baseline.md) | Partially Superseded（Issue #85、配置先だけを下記core構成へ置換。原文保持・OKF後段の境界は維持） |
| 意思決定 | [原稿の配置を本家core構成へ揃える](decisions/2026-09-04-aidlc-core-layout.md) | Accepted（Issue #87、本家準拠を基本とし、140件をsrc/coreへ無変更移動する直接承認） |
| 調査 | [既存AI-DLCの配布形式](research/2026-08-29-existing-distribution-format.md) | Current for local v2.6.123 snapshot |
| 調査 | [共通space読み取りの参照契約](research/2026-08-31-space-reading-contracts.md) | Current for local v2.6.123 snapshot |
| 調査 | [Space作成の参照契約](research/2026-08-31-space-creation-contracts.md) | Current for local v2.6.123 snapshot（U+0130小文字化の追加調査を含む） |
| 調査 | [Space一覧CLIの参照契約](research/2026-08-31-space-list-contracts.md) | Current for local v2.6.123 snapshot（public parserとsession選択の境界） |
| 調査 | [Space切替の参照契約と保存API](research/2026-09-01-space-switch-contracts.md) | Current for local v2.6.123 snapshot（共有cursor・後続処理の保存先、Go1.26.4保存境界） |
| 調査 | [Intent一覧CLIの参照契約](research/2026-09-01-intent-list-contracts.md) | Current for local v2.6.123 snapshot（registry・directory相関、表示、public parser境界） |
| 調査 | [Intent切替CLIの参照契約](research/2026-09-01-intent-switch-contracts.md) | Current for local v2.6.123 snapshot（対象解決・cursor・session副作用・Go保存境界） |
| 調査 | [Intent作成coreの参照契約](research/2026-09-01-intent-create-contracts.md) | Current for local v2.6.123 snapshot（UUIDv7・registry・Bun Windows ICU / Go Unicode overlay・部分失敗） |
| 調査 | [読み取り専用ワークスペース分析の参照契約](research/2026-09-02-workspace-detection-contracts.md) | Current for local v2.6.123 snapshot（root signal・nested depth 3・言語閾値・framework/build・submodule） |
| 調査 | [Stage graph・scope routingの参照契約](research/2026-09-02-stage-routing-contracts.md) | Current for local v2.6.123 snapshot（runtime転置・scope metadata境界・fail-closed差分） |
| 調査 | [Scope metadata readerの参照契約](research/2026-09-02-scope-metadata-contracts.md) | Current for local v2.6.123 snapshot（frontmatter block-first・ECMAScript whitespace・raw改行とinner backtrackingを含むblock regex境界・validation・3件の意図的差分） |
| 調査 | [初期 state builderの参照契約](research/2026-09-02-initial-state-builder-contracts.md) | Current for local v2.6.123 snapshot（canonical state・JSON.stringify互換sidecar・routing・Greenfield補正） |
| 調査 | [4層Memory source readerの参照契約](research/2026-09-02-memory-source-reader-contracts.md) | Current for local v2.6.123 snapshot（4層fixed path・fresh read・UTF-8 fail-closed・Root境界） |
| 調査 | [Memory bundle filterの参照契約](research/2026-09-02-memory-bundle-filter-contracts.md) | Current for local v2.6.123 snapshot（substantive判定・ECMAScript trim・comment除去・preamble） |
| 調査 | [Stage catalog metadataの参照契約](research/2026-09-03-stage-catalog-metadata-contracts.md) | Current for local v2.6.123 snapshot（成果物・consume・依存edge・runtime/compiler境界） |
| 調査 | [Intent開始時Stage Planの参照契約](research/2026-09-03-stage-plan-contracts.md) | Current for local v2.6.123 snapshot（Plan解決・依存advisory・runtime再読込境界） |
| 調査 | [StartIntentとin-flight recomposeの参照契約](research/2026-09-03-start-intent-recompose-contracts.md) | Current for local v2.6.123 snapshot（initializer seam・partial Intent・state suffix source・future recompose境界） |
| 調査 | [保存済み state readerの参照契約](research/2026-09-03-state-reader-contracts.md) | Current for local v2.6.123 snapshot（canonical state・strict parse・Root境界・意図的差分） |
| 調査 | [Current directive resolverの参照契約](research/2026-09-03-current-directive-contracts.md) | Current for local v2.6.123 snapshot（Branch 10・terminal 2形・suffix authority・意図的差分） |
| 調査 | [Stage completion artifact presenceの参照契約](research/2026-09-03-stage-artifact-presence-contracts.md) | Current for local v2.6.123 snapshot（通常Stage any-of存在、filename例外、段階的境界） |
| 調査 | [薄いライフサイクルのreport・approval・state遷移契約](research/2026-09-03-thin-lifecycle-transition-contracts.md) | Current for local v2.6.123 snapshot（guard順、marker、audit-first、state advance、PR6接続確認） |
| 調査 | [Intent候補列挙・現在intent解決の参照契約](research/2026-08-31-intent-reading-contracts.md) | Current for local v2.6.123 snapshot |
