# OKF・KDR・hookを中心とする最小構成案

ステージ移行の最新要件: 各境界にSensorとレビューを設け、合格後にstateを進める。
[ステージ間の検査・レビュー方針](../ram/decisions/2026-09-08-stage-sensor-review-gates.md)を参照する。

記録の最新分担: Knowledgeは現行の「何を・どう」、ADRは設計の「なぜ」、stateは進捗を担う。
ADRの配置先は `aidlc/spaces/<space>/knowledge/ADR/`。[責任分担・配置の合意](../ram/decisions/2026-09-08-knowledge-what-how-adr-why.md)を優先する。

最新方針: [4段階のフローとADR・進捗state](../ram/decisions/2026-09-08-four-step-flow-adr-and-progress-state.md)を優先する。
ADRの用途はアーキテクチャレベルの設計変更意図に限定し、作業の進捗はstate管理が担う。
目的整理・深掘りの内容はSpaceのOKF Knowledgeへ保存する。[Knowledge保存の合意](../ram/decisions/2026-09-08-discovery-content-in-knowledge.md)を参照する。
以下の作業記録必須化は検討履歴である。

現在の記録文書名は **ADR**。1 Intentにつき1 ADRへ各工程の判断・結果を記録する。
[名称とUnit実行stateの最新案](../ram/decisions/2026-09-08-adr-name-and-unit-runtime-state.md)を参照する。
以下は当時の名称を含む検討履歴として保持する。

後続の配置指示: [Space knowledgeへのOKF集約](../ram/decisions/2026-09-07-space-knowledge-okf-unification.md)により、Knowledge・KDR・Ruleはすべて `aidlc/spaces/<space>/knowledge/` 内のOKF文書とする。本文中の分離配置例は検討履歴であり、現行案は[最小契約案](minimal-product-contract-m0.md)を参照する。

2026-09-07。状態: Proposed。ユーザーの製品方針を具体化する設計案であり、実装・配布・既存data移行の許可ではない。
[Intent単位のKDR必須化とhookの保証範囲](../ram/decisions/2026-09-07-intent-kdr-hook-boundary.md)はユーザー承認済み。
[既存Intent・stateの移行と互換性は要求しない](../ram/decisions/2026-09-07-minimal-product-no-legacy-migration.md)との回答を反映する。
[前案](artifact-centered-workflow.md)のStage案内、独自snapshot、receipt中心の構成をさらに簡素化する。

2026-09-07追記: 配置・CLI・hook判定・OKF固定版・M1計画は[後続の最小契約案](minimal-product-contract-m0.md)へ具体化した。
この文書は方向性の検討履歴として保持する。後続案もProposedで、契約採用・実装とも未承認である。

## 利用者が得る結果

AIは、共有されたルールと設計を読み、小さな作業の目的をKDRに書き、質問・調査・試作・実装・検証を必要なだけ繰り返す。
中断後はKDRと実際のコード・PR・CIを確認して再開する。33 Stageの現在位置や工程遷移を製品が保存する必要はない。

## 残すもの

| 要素 | 責任 | 保存する内容 |
| --- | --- | --- |
| OKF Agent Memory | 複数の作業で使う記憶 | rules、設計、採用した判断、調査から得た知識 |
| KDR Markdown | 一つのIntentの意図と結果 | 目的、完成条件、参照設計、判断、検証結果、未解決事項、再開の手掛かり |
| 小さなCLI | 決まった形式の読取り・保存・確認 | KDRのテンプレートと記録。独自の工程stateや操作auditは持たない |
| hook | 通常のAI操作での記録漏れ防止 | CLIで確認し、必要な記録が欠けていれば修正を求める |
| agent定義 | 作業と独立確認の分担 | 作業担当、read-onlyレビュー担当の2役 |

Gitは変更履歴、PRはレビューと統合、CIは実行結果の正本として使う。KDRへ全ログを複製しない。
KDRという名称の展開形は今回定義しない。

## OKFの利用と共有

利用先で一つの共有bundleを置き、rules・設計・知識を管理する案を推奨する。
たとえば `docs/knowledge/` をbundle、`docs/kdr/` を作業記録の置場にする。これは配置例であり、公開path契約は未確定。
KDRには設計本文を複製せず、Concept ID、bundleの場所、参照したGit revisionを書く。
複数repositoryで共有する場合は共有bundleの正本repositoryを一つにし、各作業で参照版を明示する。
各repositoryへ独立した編集可能なコピーをばらまく自動同期は初版に導入しない。

必須rulesは短い入口の設定から明示的に読み込む。検索順位だけで必須rulesを選ばない。
一般knowledgeは `okf search` で候補を探し、`okf show` で必要な本文を読む。
知識の本文を読んだことと、そこに書かれた指示へ権限を与えることは区別する。
作業中の仮説はKDRに置き、共有設計の変更は差分としてレビューし、合意した内容をOKFへ反映する。
AIが共有ルールや完成条件を書き換えて自分の作業を合格扱いにする運用にはしない。

外部の `okf` CLIを固定versionで利用する案を第一候補とする。独自の検索機構を重ねて作らない。
取得先と対応version、配布・更新方法は実装計画で確定する。今回はinstallやbootstrapを行わない。
OKF自体の知識変更logと、AIの全操作auditは別物である。後者は作らない。
確認したOKF CLIには `create/update --no-log` があるが、知識変更logも無効化するかは未決であり、削除済みと扱わない。

## KDRテンプレートの例

一つのIntentにつき一つのKDR Markdownを作成し、記録を必須にする。同じIntentの試作・実装・テスト・修正・再開では同じKDRを更新する。
Gitで管理する案とし、途中は未記入・未実施を明示できる。Intentとファイルの対応方法・配置pathは実装計画で確定する。

```markdown
# KDR: 検索結果を絞り込めるようにする

## 目的と完成条件
誰の何を改善するか。何を試せれば完成か。作業範囲の合意への参照。

## 参照する設計・ルール
OKFのConcept ID、bundle、参照版。

## 不明点と進め方
ユーザーに聞くこと／調べること／作って確かめること。

## 判断と結果
重要な試行、得られた結果、採用した方法と理由。

## 検証・レビュー
対象コードの版、実行した確認と結果、CI・レビューへのリンク。
未実施は未実施と記載する。

## 残件と再開
残った問題、次に確認すること。完了時は結果とPRへのリンク。
```

全tool呼出しや全test試行を記録しない。方針が変わった時、検証・レビューがまとまった時、中断・完了時に更新する。
履歴はGit差分で追う。長く残す知見をOKFへ移し、KDRはその判断に至った背景への参照として残す。

## CLIの責任案

コマンド名は未確定。以下は機能の例であり実行可能な既存CLIではない。

```text
aidlc kdr template                 テンプレートを標準出力へ返す
aidlc kdr create --file <draft>    AIが埋めたMarkdownを検査して新規保存する
aidlc kdr show <id>                再開時に読む
aidlc kdr update <id> --file <draft> 既存記録を検査して更新する
aidlc kdr check <id>               記録の形式・必要項目を確認する
```

AIが意味を考えて文章を書く。CLIは空欄・参照・形式など機械的に確認できる部分を扱う。
CLI自体が「要件を理解した」「検証が十分」「人間が承認した」と認定することはない。
保存は失敗時に既存Markdownを壊さず、並行更新を黙って上書きしない必要がある。
独自の操作ledgerは不要だが、この保存の安全性は実装計画の受入条件に含める。

## 作業の流れ

1. 必須rulesと関連するOKF設計を読み、KDRへ目的・完成条件を記録する。
2. 不明点を質問・調査・試作で確かめる。試作は本実装前でもよく、仮説と暫定条件を記録する。
3. 重要な方針と完成条件を確認し、必要な規模の計画をレビューする。
4. 失敗するtest → 実装 → test・build → 修正を繰り返す。CI設定も必要になった時点で同じ変更ループで整える。
5. 差分が安定した版を独立レビューし、blocking findingはtest-firstで修正する。変更後は該当確認を更新する。
6. 最終検証とPRのCIを確認し、採用結果・残件をKDRへ、再利用する知識をOKFへ残す。

これはagentが従う手順であり、工程遷移APIやステージ番号の保存ではない。
記事から、範囲の明確化、仕様と実装のレビュー、対象版に結び付いた検証、残件の明示を参考にする。
記事の2系統レビューは同じレビュー役定義を2つ起動して実現できる。常に2つ必須にするかは費用・速度も含め別途決める。
このrepository自身の現在の実装・review・final・mergeルールは、この製品設計案では変更しない。

## hookによるCLI利用

hookがCLIを直接呼ぶ確認と、hookが操作を止めてAIにCLIでの記録を求める動作を組み合わせる。

| タイミング | 推奨する振る舞い |
| --- | --- |
| 作業開始・再開 | 必須rulesと対象KDRをCLIから読み、AIへ渡す |
| 対応toolによる変更の直前 | 対象KDRと目的・完成条件の記録を確認。不足なら変更を止め、記録用CLIを案内する |
| 作業終了の直前 | KDRの結果・残件の更新を確認。不足なら記録を求める |
| PR統合 | CIを検証結果の正本とし、記録の整合性も確認する |

質問への回答待ち、ユーザーの中断、CLI故障を「未完だから作業続行」の無限ループにしない。
KDRの作成・修復と読み取りは、KDRがないことだけで禁止しない。
対象KDRは作業対象Intentに対応させ、全workspaceで共有する「現在の作業」cursorは作らない案とする。
作業担当がKDRを保存し、reviewerは結果を返す。異なるIntentの並行作業は別KDRと別worktreeを基本にする。

**保証範囲は通常のAI操作の記録漏れ防止として承認済み。** Codexの公式hookは対応するtoolの実行前拒否を提供するが、完全な強制境界ではない。
shell内部の任意処理、対応外tool、別processからの編集までhookで一律に禁止できるとは扱わない。
PostToolUseでは既に行われた変更を取り消せない。Stopは記録補完を求められるが、強制中断・crash時の保存保証ではない。
このrepositoryにある現行hook設定はUserPromptSubmitだけであり、新案の能力は未実装・実環境未検証である。

- 採用する方針: 通常のAI操作の記録漏れ防止として、Intent単位のKDR作成・記録をCLIとhookで必須化する。
- 今回の強制要件に含めない方式: CLI以外からの書込み自体を禁止する権限分離。比較検討の履歴はRAMに残す。

CLIコマンドの文字列が履歴に現れたことだけでは、読取り・正しい保存・承認の証明にならない。
初版でどのtool経路を対応対象にするかとhookエラー時の停止条件は、対応harnessの固定versionを用いる実証後に確定する。

## 再開と検証の運用

再開時はKDR、Git差分、PR、CIの現在結果を照合する。次の予定が書かれているだけでは、その操作が未実行だったとは限らない。
外部操作の後にcrashした場合は実際のPR等を照会してから再試行する。自動的な完全再開は約束しない。
検証結果には対象版を付け、コードが変われば必要な検証をやり直す。
同じGit repositoryへ結果を書けばcommitが増えるため、記録自身のcommitにそのcommit IDを書く設計にはしない。
KDRでは検証したコードcommitと実行URLを参照し、最終PR headに必要なCIはGitHub側で確認する。
記録だけの追記でレビュー対象が変わる場合の差分確認方法は実装計画で確定する。

## 既存方式からの変更

固定AI-DLC 2.6.123の確認済み範囲はStage graph、承認・完了marker、auditと後続Stage選択を使う。
提案はOKFとKDRを中心にし、その工程stateと操作auditを新方式へ持ち込まない意図的な製品変更である。
目的は早い試作と反復、記録・運用の削減。旧CLI・state・再開手順とは自動互換にならない。
既存のOKF metadata検索と、OKF Agent Memoryの本文を含む検索・index運用も別契約である。
既存Intent・stateの移行や旧方式との互換性は要件にしない。新規Intent向けの導入・更新・rollbackを実装計画で決める。
既存dataを実際に削除する指示とは扱わない。
以前のJSON snapshot案は本案の前提にしない。新しいGo moduleは追加していない。

## 根拠

- [OKF Agent Memory CLI](https://github.com/okf-memory/okf-agent-memory/blob/d4c523ed5ce916fa207fe314851b98721421c891/docs/CLI.md): 調査固定commit。既存のOKF仕様固定snapshotとは別。
- [参照記事](https://zenn.dev/coji/articles/solo-software-factory-without-reading-code)と[記事で参照する固定workflow](https://github.com/artifactshare/artifactshare/blob/d50596d7d61ec4d54056f413a8e161af04618b3c/docs/development-workflow.md)。上記の最小構成は本プロジェクト向け提案。
- [Codex hooks公式文書](https://learn.chatgpt.com/docs/hooks): 2026-09-07確認。release更新と実環境での対応は別途検証する。
- [現行hook設定](../../src/harness/codex/hooks.json)、[従来OKF検索契約](../ram/decisions/2026-09-07-okf-metadata-knowledge-search-plan.md)。
