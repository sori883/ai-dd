# プロジェクトRuleとAI-DLCスキル分離の実装計画

## 背景と実装許可

利用プロジェクトの共通ルールを置くrule.mdに、AI-DLC自身の進行・承認・記録規約が混在し、WORKFLOW.mdとskillにも操作案内が重複している。利用者がプロジェクトのルールを管理しやすくするため、rule.mdをその用途に限定し、進行をaidlc、操作案内を新aidlc-cliへ移す。ユーザーはこの具体的な責任分担とWORKFLOW廃止を明示承認済み。許可根拠は[直接承認RAM](../ram/decisions/2026-09-10-project-rule-and-cli-skill-approved.md)。この計画は承認された分離に必要な配布・参照・既存検査の接続を具体化する。

## 利用者が得る結果

プロジェクトRuleへ言語・命名・設計制約などを記載する。メインAIはaidlcスキルで進行・承認・記録先を確認し、aidlc-cliで操作を探してCLI helpで正確な引数を確認する。各stageの作業とagentの責務は対応する定義を読む。Knowledgeは現行what/how、ADRはwhy、進捗はstate、差戻し理由はwork-logという意味を保持する。

## 配置と検査

- src/core/minimal/knowledge/rules/rule.md: type Rule、初期titleはプロジェクト共通ルール。本文は追加ルールなしと明示。利用者未指定の制約を作らない。entry.mdのlink表示名を合わせる。
- src/core/workflow/stages/*.md: Ruleは既存Reference形式で ${knowledge_root}/rules/rule.md + metadata type Rule を参照。titleを製品固有文字列で固定せず、type/title/description/本文の検証を維持する。既存固定pathを正本にできるため検索件数の意味を新設しない。
- src/harness/codex/minimal/SKILL.md: AI-DLC全体の規約と共通記録、担当依頼、現在手順取得。4 KiBの既存bootstrap容量内。詳細を無条件にcontextへ追加しない。
- src/harness/codex/minimal/aidlc-cli/SKILL.md: 新CLIスキル。@@BINARY@@参照、操作目的からhelpへの索引、操作の注意。配置先 .agents/skills/aidlc-cli/SKILL.md。
- WORKFLOW.md: 規約をskill/工程/agentへ、必要な操作仕様はhelpへ移管してから原稿/embed対象/新配布から削除。単なる全文コピーや型定義の重複を作らない。
- install.go: 明示mappingで両skillを配置。既存配置の事前検査/無上書きを維持。
- relocate.go: 両skillとhooksの3fileを全件事前検査し、既知binary参照だけ置換。部分失敗Paths/Pending、同一retry、未知編集/欠落で無変更停止を維持。旧版を新版へupgradeしない。
- minimal/hook.go: 限定cat読取りを両skillへ対応。未読Rule・承認待ち・in-flight等の既存gate順、redirect/複合command/任意path/symlinkの拒否を保持。
- flow/procedure.go: initializationに新skillの配置検査を追加。欠落を埋込み版で隠さない。
- cli help: 移管が必要な操作仕様だけを整理。引数や保存形式を新設しない。
- okfmemoryの既存parserを使い、SpaceのRuleコピー・全文/hash照合を維持する。

初期の追加制約なしは非空本文として有効とする。空本文やRule欠落を通す緩和ではない。schema・承認・Sensorの意味は変更しない。定義hash変更後は従来どおり新配布/新Intentで利用し、既存利用先のRule・旧WORKFLOW・編集済みskill・stateを自動移行/上書き/削除しない。

## 所有範囲と順序付きTDD

1 Issue/PR、単独writer、work_unit_id=project-rules-and-aidlc-skills、verification_mode=loop。所有は上記原稿・stage/必要agent参照、src/internal/install,flow,minimal,cliと関連workspace/okfmemory/cmd tests、docs/development.md、当該RAM/索引/証拠。原本checkout /Users/const/sori883/ai-dd は編集禁止。作業は /Users/const/sori883/ai-dd-stage-okf-documents の独立branchで行う。

1. 配布/責務/旧参照なし/bootstrap容量: go test -count=1 ./src/internal/install -run '^TestRuleSkillSeparationAssets'
2. 利用者title/制約なし/欠落不正/Rule全文hash/Spaceコピー: go test -count=1 ./src/internal/flow ./src/internal/minimal ./src/internal/workspace -run '^TestRuleSkillSeparationRule'
3. 両skill限定読取り/待機/危険command拒否/未読Rule・in-flight維持/配置欠落: go test -count=1 ./src/internal/minimal ./src/internal/flow -run '^TestRuleSkillSeparationHook'
4. 3file移転/事前検査/失敗retry/旧配置保全: go test -count=1 ./src/internal/install -run '^TestRuleSkillSeparationRelocate'
5. helpへの到達/必要な詳細/fixture検証器: go test -count=1 ./src/internal/cli ./src/cmd/aidlc -run '^TestRuleSkillSeparationHelp'

各項目test-first、runnable REDまたはALREADY_GREENを記録する。文言に人工REDを作らず、配布・参照・保全・gateを検証する。既存fixtureの旧Rule/WORKFLOW参照を正規の新配置へ追従し、検査を緩めない。末尾にtargeted群/gofmt/diff確認、親boundary確認、独立review。

## 最終検証とリスク

安定HEADで全test/race/vet、format/tidy/diff、全integration（配布journey含む）、6OS/arch構成build、native CLI/helpを確認する。新配布の両skillをskill-creator quick_validate.pyで検証する。既存固定Codexで限定live: aidlc→aidlc-cli→help/procedureの実読込、プロジェクトRule読込み、文書のCLI保存、承認待ち非迂回を確認する。必要な既存memory/human-approval live fixtureは新配置へ追従する。モデルへの説明が失われるリスクは独立reviewで移管元と照合しliveで確認する。固定head後に修正した場合は必要targeted/reviewからfresh finalへ戻る。

Go単一binaryと標準ライブラリ、外部module/tool導入なし。GitHub checks成功後にmerge commitで反映しIssue closeを確認する。新配布は旧Intentへ暗黙に適用せず、必要な場合は元版を維持することで復帰する。重大未決事項なし。
