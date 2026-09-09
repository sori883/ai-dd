# ステージ別担当の起動制限と同一worktreeのworker排他を計画する

状態: Requested / Proposed。2026-09-10。**計画のみ。製品実装・実機実験は未承認。**

## ユーザー要求と承認境界

現在ステージに登録されていない担当の`spawn_agent`を起動前に拒否し、同じworktreeで複数のworkerが同時に稼働することを防ぐ。全サブエージェントの人数制限にはせず、別worktreeの並列実装を維持する。担当範囲外の全ファイル編集制限は対象外。外部Go moduleを追加せず、既存のGo単一バイナリを維持する。

ユーザーは未検証のCodex前提を計画のgateにし、重要な未決がある間は今回は実装しないよう指定した。旧ロードマップや既存機能の承認を新しい起動制限へ流用しない。

## 現在地と確認結果

元checkoutは`f0e97d4504ffdffa474abf31082626711f954d63`で未commit資料を保持。mainはPR #158の`c990f7629c69dd0a3f11ad1994dd69c165e70749`でOpen Issue/PR各0件。mainと一致する既存worktreeを計画の基準にした。

現行hookはBash/apply_patch。Unit claimの重複検査は同一Intentのrunning/needs_confirmationに限られ、reportedは実子の停止を意味しない。Session.Toolも子の寿命に使えない。段階の担当一覧は既存Procedure.Agentsを正本とする。mainではdiscoveryにstage-plannerが追加済み。

公式hook文書とContext7を確認した。公開仕様上はPreToolUseでspawnを拒否でき、SubagentStartでは起動を止められない。SubagentStopだけから残存process終端を判断する根拠は不足している。手元のCodex CLI 0.153.4のversionのみ確認し、今回のテスト・起動実験は未実施。過去の担当起動smokeは拒否/排他の実証ではない。

## 提案と未確定事項

[計画](../../design/stage-agent-worker-guard-plan.md)へ、起動前の担当照合、調整root共通registry、実要求/子ID/rootの構造化対応、世代付き予約、state変更との競合、保存故障、復旧、配布・信頼・rollback、単独writer/TDD/検証を記載した。

registry候補は調整rootの`aidlc/.runtime/agents/registry.json`。報告・spawn Post・親Stop・経過時間だけで解放しない。実機の対応関係と停止/再開の捕捉が成立しなければ製品実装へ進まない。このruntime案は承認済みの永続形式ではない。

ユーザーへ次の3点を確認中であり、回答待ち。

1. 途中の計画変更を維持するため、stage-plannerを各段階のagentsへ明示追加するか、discoveryだけに限定するか。
2. Unitなしの子workerに実行予約を用意するか、子worker使用時は1 Unit以上を必須にするか。
3. 既存合意の1調整rootへ集約するか、複数の調整rootが同じworktreeを使う場合も共有予約で保証するか。

同一調整rootのregistryだけでは、別調整rootとの排他を保証できない。ユーザーの人数制限要求を全agentやメインAI自身へ広げない。自然言語promptからrootを推測せず、専用起動経路が必要なら既存scheduler非要求との関係を再承認する。

ローカル固定本家2.6.123のCodex adapter/配布hook/子終了記録のみを確認し、全scheduler・全ハーネス・最新upstreamは未確認とした。過去のRAMは削除・上書きしない。

read-onlyの計画点検で、Q1のagents変更によるdefinition hash更新と既存Intentの互換性を補足した。既存Intentを新定義へ暗黙に再結合せず、対応旧定義の維持/復元または新Intentで扱う。新Intentを残して旧定義へ戻すrollbackもhash不一致で停止する。これは既存の定義固定境界の継続であり、自動移行の承認ではない。

## 今回の成果と次のgate

今回の変更は計画、本RAM、索引のみ。コード・製品設定・Issue・PRの変更、commit、test、実機実験は行わない。Q1〜Q3の回答とG0実験の承認・結果を記録してから契約を確定し、製品実装の明示承認へ進む。
