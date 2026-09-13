# 外部由来skillを原典の名前で配布する

## 目的と実装許可

AIに渡す作業手順であるskillのうち、外部プロジェクトから翻案したものにも`aidlc-`を付けていた。
利用者に本製品の独自作成物と誤解させないため、原典名と作者・出典を明確にする。
2026-09-13の「独自のskillsじゃないものは、ai-dlcという接頭語を付けないで」というユーザーの直接修正依頼を根拠とする。
これは共通化後の命名・帰属表示の限定修正であり、外部原典の手順を新たに丸ごと採用する依頼ではない。

## 名前と帰属

| 変更前 | 配布名 | 原典 |
| --- | --- | --- |
| aidlc-grill-with-docs | grill-with-docs | mattpocock/skills |
| aidlc-grilling | grilling | mattpocock/skills |
| aidlc-domain-modeling | domain-modeling | mattpocock/skills |
| aidlc-research | research | mattpocock/skills |
| aidlc-to-spec | to-spec | mattpocock/skills |
| aidlc-tdd | tdd | mattpocock/skills |
| aidlc-code-review | code-review | mattpocock/skills |
| aidlc-architecture | architecture | owainlewis/blueprint |
| aidlc-planning | planning | mblode/agent-skills |
| aidlc-systematic-debugging | systematic-debugging | obra/superpowers |
| aidlc-verification-before-completion | verification-before-completion | obra/superpowers |
| aidlc-okf | okf-agent-memory | okf-memory/okf-agent-memory |

外部由来の`natural-japanese-go`は既に接頭辞がなく、Go移植を示す名前を維持する。
13個の翻案skillの本文冒頭から原典・作者と、この製品向けに翻案した事実が分かるようにする。
既存の固定commit、LICENSE、参考本文を保持し、元の作者による公式配布や動作保証と誤認させない。
独自の`aidlc`・`aidlc-cli`、任意の`aidlc-github`、製品の5担当名は変更しない。

11工程skillの原典名は既存`references/source.md`から確認した。
OKFは固定`a09e04918aa84d275b784374b5236d9eeac56c9e`の
[原典skill](https://github.com/okf-memory/okf-agent-memory/blob/a09e04918aa84d275b784374b5236d9eeac56c9e/.agents/skills/okf-memory/SKILL.md)
のfrontmatter `name: okf-agent-memory`を使う。原典のディレクトリ名`okf-memory`と混同しない。
OKFにも`references/source.md`と原典LICENSEを追加する。取得したLICENSEのSHA-256は
`8de41d98bcdfca0d6a0399820f4d79e0738ff53a5fb04d605d1ecad7871cbafb`。
手順の検索・保存方法は本製品の`aidlc memory`を維持する。

## 変更範囲と所有権

main `617756dcdb44daf1c533497b8b2a0bf50a978acb`、作業先`/Users/const/sori883/ai-dd-release`、
branch `codex/upstream-skill-names`。work unitは`upstream-skill-names`、実装writerは1人。

- `src/core/skills/`：表の12フォルダとfrontmatterのnameを変更。相互リンク・帰属表示・OKF出典とLICENSEを整備する。
- `src/core/agents/`、`src/core/workflow/stages/`：skill参照だけを変更する。`aidlc-researcher`等の担当名を部分一致置換で壊さない。
- `src/internal/install/relocate.go`、`src/internal/flow/procedure.go`、`src/internal/workflow/definition.go`、
  `src/internal/cli/help.go`と関連test：新しいOKFの配置先、初期化の必須読取り、helpを一致させる。
- `src/harness/codex/`の関連test、`src/internal/{install,app,flow,cli,workflow}`の関連test、
  `src/cmd/aidlc/*integration_test.go`：完成資材の列挙・hook読取り・移転・実機fixtureの参照を追従させる。
- `src/docs/`、`docs/{architecture,distribution,development,developer-references-and-dependencies}.md`、
  `.agents/skills/aidlc-github/`と`src/docs/optional-skills.md`の該当参照：現行の名前と帰属の説明を合わせる。
- 本計画・新RAM・索引：親が承認根拠を記録し、writerがloop証拠を追記する。過去RAMと過去計画は書き換えない。

旧69資材のhash fixtureは履歴として保持し、新しい配布のfixtureを別に作る。
旧版との差分が命名・帰属・その参照だけであること、LICENSEが変わらないことを確認する。
固定hashの期待値を無条件に除外して検査を通す方式にはしない。

## 順序付きTDDと受入条件

| 順序 | 観測する内容 | loop command |
| --- | --- | --- |
| 1 配布名 | 15skillが新名で配置され、旧名は配置されない。nameとフォルダ一致、13skillの帰属・LICENSE・参照が届く | `go test -count=1 ./src/internal/install -run '^Test(UpstreamSkill|StageSkills|NaturalJapaneseSkill)'` |
| 2 利用接続 | 新OKF必須入力・読取り・移転が成立し、旧pathや未知pathの誤許可、保存失敗時の緩和がない | `go test -count=1 ./src/internal/app -run '^Test(UpstreamSkill|OKFSkillRead|StageSkillsRead)'`、`go test -count=1 ./src/internal/flow -run '^Test(UpstreamSkill|OKFSkillInitialization)'`、`go test -count=1 ./src/internal/install -run '^Test(Relocate|OKFSkillRelocate)'` |
| 3 参照と定義 | 完成workflow・担当・helpに旧skill参照がなく、担当名・工程の入出力・Sensor等は変わらない。新しい固定fixtureと比較 | `go test -count=1 ./src/internal/install -run '^Test(CodexManifest|ProductAgentAssets|UpstreamSkill)'`、`go test -count=1 ./src/internal/flow -run '^TestProcedureBoundary'`、`go test -count=1 ./src/internal/cli -run '^Test(RuleSkillSeparation|CheckHelp)'` |
| 4 証拠fixture | 実読取りの記録を新名で照合でき、未読・異なる本文の成功扱いを避ける | `go test -tags=integration -count=1 ./src/cmd/aidlc -run '^Test(StageSkillsEvidence|MemoryMetadataCommandEvidence)$'` |

各sliceで先にtestを追加・変更し、実行可能な期待値不一致のREDから最小GREENへ進む。
既に成立する条件はALREADY_GREENとする。文章の帰属追記だけへ人工REDを要求しない。
同じ名前の既存skillがある場合は従来どおり新規配置を拒否し、利用者の原典skillを上書きしない。

独立reviewは命名の残存、作者・原典・翻案の区別、参照整合、fixture差分、hookと初期化の維持をread-onlyで確認する。
差分安定後のfinalは全packageの通常test・race・vet、`gofmt -l src`、`go mod tidy -diff`、`git diff --check`、
workspaceとOKFのintegration、FlowJourney／GitIndependentJourney／RelocationCommand／StageSkillsEvidence、
DistributionJourney／NaturalJapaneseDistributionJourneyを実行する。
新規配置15skillのvalidatorと生成TOML／Codex設定ロードを確認し、PRの全CI・配布checks成功後にmergeする。
今回の変更で実agentの権限や起動方式は変わらず、前回の実機結果を新しいskill名の実読取り成功とは扱わない。

## 利用条件と復旧

新規配置先のskill名と工程内リンクが変わるため、工程定義のhashも変わる。
旧共通化計画の工程bytes維持はこの名称修正部分について置換する。新規配置・新規Intentを対象とし、
既設の工程定義や保存stateを自動で書き換えない。移転は引き続き同一版の3skill＋hooksの絶対path修正だけである。
既存ファイルは保持し、元checkoutへ変更を適用しない。復旧はPR revertまたは対応する旧版の新規配置を使う。
外部Go module、CLIの機能、工程の許可担当、Sensor・承認・保存形式を変更しない。
本家AI-DLCの新しい工程挙動差分は加えない。外部原典への帰属を明確にするもので、法的適否の断定は行わない。
