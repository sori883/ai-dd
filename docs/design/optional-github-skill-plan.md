# 任意導入のGitHub Issue・PR skill

日付: 2026-09-12。実装許可は「githubのissue、prのスキルも作ってほしい」「ai-dlc本体に組み込まず、あくまでスキルレベルでプラグイン的に」「intentやboltの作業単位でissueを作って完了時にprを出す」「作業は実施して良い」という直接依頼。

## 背景と得られる結果

AI-DLCは、一つの目的をIntentとして管理する。実装を分ける場合のUnitは担当する作業、BoltはそのUnitをまとめる単位である。
現在の製品はGitを必須にせず、進捗・承認・検証結果をローカルで管理する。リポジトリ内の既存github-pr-workflowは、ai-dd自体の開発用で、専用ラベルや自律マージの規則を持つ。

今回は利用プロジェクトへフォルダ単位でコピーできるaidlc-githubを作る。選んだIntentまたはBoltについて、開始時にIssueで目的と受入条件を共有し、対象の実装・検証・レビューが揃ったらPRで変更を提出する。IssueやPRのリンクから会話を再開でき、同じ作業を重複作成しにくくする。

基準mainはPR #180のabd08b8082bac4becbd4b3fc8faa1e0aee4b3466。開始時Open Issue・PRは各0件。Issue #27／PR #28の既存skillとは用途を分ける。作業場所はai-dd-naming、branchはcodex/optional-github-skillとし、元checkoutの未commit資材を保持する。

対応する開発Issue: [#181](https://github.com/sori883/ai-dd/issues/181)。分類はユーザーリクエスト。

## 具体的な契約

- 原稿は`.agents/skills/aidlc-github/`。利用先も同じdiscovery pathへコピーする。現在のsrc/harness/codexのembed、install、stage、agent、hook、CLI、stateは変更しない。Codex pluginのmanifestや自動導入機能も追加しない。
- 通常はIntent単位。承認済み計画や利用者指定がBolt単位なら、その実行回のBoltに属するUnitをまとめる。一律にIntent親Issueと全Bolt子Issueを両方作らず、Unitごとにも自動分割しない。採用する単位をIssue作成前に明示する。
- GitHub操作をするメインAIが、Space、Intent ID、対象repository、対象のGit作業場所、公開してよい範囲を確認する。base/headがIssue作成時に未確定なら未確定事項として記載し、commit/push・PR作成前に確定する。Boltはstep_idとbolt値も使い、同名Boltの別実行回と区別する。子は草稿と証拠を返す。
- 対応はIssue／PR本文のIntent ID markerと項目表、および任意のOKF文書`design/INTENT_ID/github-links`へ保存する。typeはGitHubLinks、intent_idとgithubタグをCLIで設定する。本文はリンクの対応表で、進捗stateや全操作auditではない。承認済み要件・計画、CLI管理のwork-logへ直接追記しない。
- 再開時は対応表、Issue URL、全状態のIssue／PRを確認する。検索結果が0件でも検索索引の遅れや件数上限があるため、通信結果不明の直後に重複作成しない。複数候補や保存途中は根拠を照合し、正本を特定できなければ停止する。
- Intentの各工程、Boltの対象repositoryのIssueに含む全Unit、対象の受入条件を区別する。他repositoryのUnitは同じ完全集合に含めないが、依存や全体検証の条件は別途満たす。Unitのreportedは提出、integratedは内容の統合であり、独立レビューや人間承認の代わりではない。PR作成でstateを完了にせず、未完了なら残件を返す。利用者が途中共有を求めた場合だけDraft PRを扱う。
- 現行hookはactiveかつbegin済みの作業を要求する。PR提出は必要な成果確認・承認を済ませ、Intent単位なら最後の工程、Bolt単位なら対象TDD工程のfinish前に行う。finishは現在のUnit一覧を消去するため、Boltの対応と成果は先に確認・保存する。completedのIntentをPR作成条件にしない。既に完了済みなら通常の再開手順について利用者へ確認し、hookを迂回しない。対応表保存が検証対象を変える場合は、変更後の証拠と必要な承認を改めて揃える。
- GitHubを使う操作にはGit・ghと認証が必要だが、本体のGit不要の運用は維持する。複数repositoryにまたがる作業はrepositoryごとのIssue／PRを対応表で結ぶ。別worktreeを必須にしない。
- PRを作る許可からmerge、Issue close、branch削除の許可を推測しない。利用先の明示依頼や承認済み運用に含まれる場合だけ実行し、保護規則・review・checksを維持する。このai-dd開発用の自律マージ規則を他プロジェクトへコピーしない。
- baseは利用先の合意したbranchを明示する。非default branchではclosing keywordによる自動closeを期待せず、Issueの完了境界を確認する。全Intentを閉じるkeywordを一つのBolt PRへ付けない。

## 確認した根拠

現行user-guide、aidlc-cli／aidlc-okf、Unit処理とmemory createの公開helpを確認した。memoryのtypeは自由な非空文字列で、日時はCLIが生成する。work-logはCLI管理。任意skillの原稿を現行embed外へ置けば標準配置と既存の15skillは変わらない。

GitHub CLIは手元の2.94.0を基準にContext7と公式manualを参照する。検索・再試行・PRのbase/headと本文ファイル、closing keywordの条件はskillの根拠リンクに残す。固定本家AI-DLC 2.6.123の既存分析は参照資料であり、最新upstreamとは扱わない。今回、本体の仕様・挙動を変更せず、任意のGitHub運用手順を追加する。

## ファイルと所有者

親エージェント一人が以下を編集する。調査、skillの独立適用、review担当は読み取り専用で、同じ作業ツリーを編集しない。

| ファイル | 内容 |
| --- | --- |
| .agents/skills/aidlc-github/SKILL.md | 使用条件、作業単位、開始・再開・PR提出の入口 |
| .agents/skills/aidlc-github/references/work-items.md | IDの対応、Issue本文、OKFでの対応表保存 |
| .agents/skills/aidlc-github/references/github.md | 対象確認、gh操作、検索・失敗時の再照合と一次資料 |
| .agents/skills/aidlc-github/references/pull-request.md | 完了条件、PR本文、任意のmergeとclose |
| src/docs/optional-skills.md | 任意導入、使い方、既存ファイル保全と取り外し |
| README.md | 任意skillの案内リンク |
| この計画、docs/ram/decisions/2026-09-12-optional-github-skill-approved.md、docs/ram/README.md | 実装許可、判断、索引 |

## 検証と完了

文書とskillの追加のみで、Goコードや設定の観測可能な動作は変更しない。人工的なREDやGo実装TDDは設けない。

1. loop: skillを単独フォルダとして一時配置し、quick_validate.py、相対参照、固定rootや未解決の配布置換文字列がないことを確認する。
2. 独立適用: 会話を知らない担当に、再開したIntentの既存Issue／不確定なPR作成結果、および複数repositoryに分かれるBoltと未完了Unitの架空資料を渡す。実GitHubへの変更は許可せず、返る本文案・次の操作・停止理由を確認する。
3. 独立review: 固定した差分と計画について、誤ったIssue close、重複作成、承認・Sensor回避、必須Git化、開発用規則の混入がないか確認する。
4. read-only final: validator、全相対リンク、配置コピーの一致、git diff --check、変更pathの限定を確認する。Go本体・既存skill・workflow・依存が基準mainから不変であることを差分で確認する。検証中の追跡ファイル変更は認めない。
5. この開発作業のIssue／PRは既存github-pr-workflowで管理する。対象PRで起動するGitHub checksが成功したら、ai-ddの承認済み運用どおりsquash mergeし、main反映とIssue closeを確認する。

受入条件は、任意skill単体を持ち出せること、Intent／Boltとrepositoryを混同しないこと、再開・不確定結果で既存項目を再照合すること、完了証拠と公開範囲を正確にPRへ記載すること、本体に変更がないこと。

外部Go moduleや新しい実行プログラムは追加しない。ghの未導入・未認証時は草稿と不足条件を返し、自動installや権限拡大を行わない。取り外しは利用者が追加したskillと採用ルールを保全して戻す方法とし、作成済みIssue／PR、Knowledge、stateは削除しない。
