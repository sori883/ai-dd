# 共通手順を一か所にまとめCodexへ配置する実装計画

## 背景・結果・承認

現在はAI-DDの共通手順まで `src/harness/codex/` にあり、承認・担当の入力・共有文書の更新責任がskill、agent、工程の複数原稿に記述されている。共通ルールの修正を各所へ転記する必要があるため、共通Markdownを唯一の原稿とし、Codex固有の接続・設定と分離する。

`aidlc install codex` は同じバイナリ内の共通原稿とCodex差分を組み合わせ、今と同じ `.agents/skills/`、`.codex/agents/`、`.codex/hooks.json`、`aidlc/` へ配置する。利用者が別の生成コマンドを実行する必要はない。共通writer契約の変更は全担当の完成指示へ反映され、TDD等の長い方法論は必要なskillを参照する。

2026-09-13にユーザーへ配置・整理・生成・検証・Claude削除の具体案を提示し、直接承認を得た。[承認記録](../ram/decisions/2026-09-13-common-skills-codex-approved.md)を根拠とする。新規Go moduleは追加しない。CLI名、保存形式、ステージ進行、Sensor、hookの権限境界、担当管理、Go単一バイナリは維持する。本家固定2.6.123の既存参照範囲へ意図的な製品挙動差分を加える計画ではない。

## 原稿の責任

- `src/core/skills/<skill>/`：aidlc、aidlc-cli、aidlc-okf、既存11工程skill、natural-japanese-goの15個。参照資料・LICENSEも同じ単位で移す。
- `src/core/skills/shared/`：承認、担当調整、入力、検証、工程の共通操作・skill注意事項。合成部品であり独立skillとして配置しない。
- `src/core/agents/aidlc-{researcher,requirements,stage-planner,worker,reviewer}.md`：役割・入力・返却内容・担当境界の正本。
- `src/core/workflow/stages/`：frontmatterの許可担当・入出力・Sensorと工程固有手順。反復する共通操作は共通部品から展開する。配布後の工程bytes・hashを保つ。
- `src/harness/codex/skills/`：CodexのSessionStart/UserPromptSubmit、native spawn/followup/send、trustとhook復旧等の接続説明。
- `src/harness/codex/agents/`：Codexのsandbox等の設定。共通本文をGoでTOMLへ安全に符号化して含める。

Go標準ライブラリと既存Manifestの完成資材を使う。合成原稿が必要な箇所はtemplateとして識別し、完成資材に未展開記号や内部部品を混ぜない。必須の承認・writer境界をリンクだけにせず直接含める。全詳細を全agentへ埋め込まず、入口skillの4096 bytes制限を保つ。共通側からharnessをimportしない。

工程skillの対応や役割説明を複数原稿へ再手入力しない。必要な対応表は既存の許可担当定義と照合し、許可担当の第二の正本にしない。テンプレート・部品の欠落や不正な参照は保存前にerrorとする。生成結果は通常build/installで得るため、生成物のcommitやgo generateの手作業を必須にしない。

## 所有範囲

作業tree `/Users/const/sori883/ai-dd-release`、branch `codex/common-skills-codex`。製品変更のbaseはmain `9114fcd97043595ab5f88cf9b63f809d5136eccd`。README見出し・既存RAM差分を保全する。work_unit_idは `common-skills-codex-composition`、実装担当は常に1人とする。

| 対象 | 作業 |
| --- | --- |
| src/core/assets.go、新content.goとtest、skills/**、agents/** | 共通原稿の内蔵・合成・重複整理 |
| src/core/workflow/assets.go、stages/*.mdと必要なtest | 同一bytesで共通操作を展開 |
| src/harness/codex/{assets,manifest,emit}.goとtest、旧原稿 | Codex差分と共通原稿から完成skill・agentを生成。旧原稿を撤去 |
| src/harness/manifest.goとtest | 既存path・重複・親file競合検査を再利用。必要な最小接続のみ |
| src/internal/install/relocate.goと関連test | raw原稿参照を通常配置と同じrendererへ変更。従来の3skill＋hooksの移転対象を維持 |
| src/internal/app/hook.go、stage_skills_test.go等 | 原稿の物理位置に依存せず、完成配布の既知Markdownだけを読取り許可 |
| src/internal/flow、src/internal/workflowの関連test | 工程bytes/hash、初期化Sensorとprocedureの回帰。保存/判定処理を変更しない |
| src/cmd/aidlc-dist/archive.goと関連test | 日本語補助CLIのREADME・licensesの取得元を共通へ移す |
| src/cmd/aidlc/stage_skills_live_integration_test.go等の関連fixture | 完成配布の実読込み証拠へ追従。通常trustの証拠とfixtureを区別 |
| docs/architecture.md、docs/distribution.md、docs/developer-references-and-dependencies.md、docs/natural-japanese-go.md、src/docs/user-guide.md | 現行原稿path・編集と配置の説明を整合 |
| 本計画、関連RAM・索引 | 親が準備。実装担当がloop末尾の証拠と必要な補足を記録 |

親が承認・Issue・Claude整理・PRを管理し、実装担当はGitHubや他treeを操作しない。Claude整理とGo実装は別treeだが、同じtreeを同時編集しない。

## 順序付きTDD

各項目は実行可能な期待値不一致のRED→最小GREEN→整理の順。新APIの型・署名・空返値だけはcompile用scaffoldとして許可する。既に満たす契約はALREADY_GREENと記録し、文書移動だけに人工REDを作らない。loopでは対象testだけを実行する。

| slice_id | 観測する結果・所有ファイル | exact targeted command |
| --- | --- | --- |
| content | src/coreの共通合成testと実装。欠落・未展開・共通変更の全利用先反映 | `go test -count=1 ./src/core -run '^TestContent'` |
| codex | harness/codexのtestと生成。15skill/5agent、Codex差分、引用符を含むTOML、既存path | `go test -count=1 ./src/harness/codex -run '^Test(Distribution|Content)'` |
| install | internal/installの配置・リンク・LICENSE・衝突・symlink・入口上限test | `go test -count=1 ./src/internal/install -run '^Test(CodexManifest|StageSkills|NaturalJapaneseSkill|ProductAgentAssets|WorkflowDefinitionFresh|Bootstrap)'` |
| relocate | 同packageのrelocateとtest。完成原稿照合・編集拒否・部分失敗retry | `go test -count=1 ./src/internal/install -run '^Test(Relocate|OKFSkillRelocate)'` |
| read | internal/appの読取りtestと接続。最大3file・未知path・Rule等の拒否保持 | `go test -count=1 ./src/internal/app -run '^Test(StageSkillsRead|OKFSkillRead|RuleSkillSeparationHook)'` |
| workflow | core/workflowとflowの関連test。工程完成bytes/hash・既存Sensor保持 | `go test -count=1 ./src/internal/flow -run '^Test(RuleSkillSeparation|OKFSkillInitialization|ProcedureBoundary)'` |
| package | cmd/aidlc-distと関連fixture。日本語CLI付属文書の梱包・完成skillの読込み | `go test -count=1 ./src/cmd/aidlc-dist -run '^Test(Archive|Product)'`、`go test -tags=integration -count=1 ./src/cmd/aidlc -run '^TestStageSkillsEvidence$'` |

存在するtest名・入口を確認し、追加testが必要なら上記prefixの配下に置く。no tests/skipを成功にしない。必要な既存fixtureの正確な期待本文更新は範囲内とし、権限・判定を弱めない。旧69file比較は不変資材と承認済みskill/agent本文変更を分けて確認し、一括更新で回帰証拠を失わない。

## Review・final・実機

独立reviewはread-only、verification_mode=reviewで行い、必須境界の脱落、Codex固有値の残存、共通化漏れ、TOML、読取り集合、工程hash、帰属表示を確認する。修正に必要なtargeted診断だけを行う。

安定後、親がverification_mode=finalを一度開始する。以下は対象fileを編集しない検証である。

```sh
go test -shuffle=on ./...
go test -race -shuffle=on ./...
go vet ./...
gofmt -l src
go mod tidy -diff
git diff --check
go test -tags=integration -count=1 ./src/internal/workspace
go test -tags=integration -count=1 ./src/internal/okf
go test -tags=integration -count=1 ./src/cmd/aidlc -run '^Test(FlowJourney|GitIndependentJourney|RelocationCommand|StageSkillsEvidence)$'
go test -tags=integration -count=1 ./src/cmd/aidlc-dist -run '^Test(DistributionJourney|NaturalJapaneseDistributionJourney)$'
```

完成skillのvalidator、生成TOMLの構文/Codex設定ロード、固定Codex CLI 0.153.4の隔離projectで実読込み・担当起動・親宛報告・既存hook拒否・日本語補助CLIを確認する。通常trustの実機と、明示trustを使う既存TestStageSkillsLiveを別の証拠として扱う。自己申告・構文成功だけを実機成功にしない。CIの3OS×2architecture build、archive照合、3OS native配布と対象PRの全checksを確認する。

## Claude整理・復旧

削除前にClaude専用treeの未保存RAMと計画を回収し、取消を後続決定へ記録する。Issue #185、local branch codex/claude-adapter、専用tree ai-dd-namingを対象とし、対象に実行中作業がないこととremote/PR状態を再確認する。調査時点でremote branch・PRはない。mainのD1・他tree・元checkoutの未commit資材を削除しない。

原稿整理はPR revertで戻せる。利用projectは新規配置で確認し、既存fileやKnowledge/stateを自動更新しない。旧binaryと新skillを混ぜた移転・移行機能へ広げない。実装途中で工程bytes維持が不可能な重要な選択が判明した場合は黙ってhashを変更しない。Claude対応・公開ReleaseはCodex確認後に別途決める。
