# 知識配信を工程へ接続する前提調査

- 日付: 2026-09-05（Asia/Tokyo）
- 状態: Current for local AI-DLC 2.6.123 snapshot
- 目的: ルール・知識配信をrun-stageへ接続する前に必要な入力と、現在の実行能力の境界を固定する

## 調査結果

### Depthの取得が必要

固定版の`core/tools/aidlc-orchestrate.ts:3184-3189`は、保存済みstateがある場合に`getField(stateContent,
"Depth")`を呼び、その値をinline knowledge一覧へ渡す。同`:2943-3060`はtrim・lowercase後が`minimal`の
場合だけ、工程・担当AI別の既知文書表で同梱知識を絞る。したがってGoの配信構成でも、開始時のscope
metadataへ戻らず、保存済みstateのDepthを読む必要がある。

### input artifactのpath解決が必要

run-stageは知識だけでなく、工程定義と前工程が生成した入力成果物も受け手へ渡す。固定版の
`resolveConsumes`は各consume名のproducerをstage catalogから解決し、Intent record root相対の
`<producer-phase>/<producer-slug>/<Filename>`へ変換する。optional条件とproject typeも評価する。

現在のGoにはstage metadataと成果物存在判定はあるが、run-stageへ渡すconsume／produce path一覧を
純粋に組み立てるAPIがない。Depth readerの次に、通常Stageのpath解決を別Issueで実装する必要がある。
per-unit、CodeKB、per-kind applicabilityは既存walking skeletonが未対応として停止する境界を維持する。

### 固定catalogの通常工程は現在のgate能力だけでは完走できない

配置済み固定版`tools/data/stage-graph.json`には33 Stageがある。最初の3件は`initialization`で、残る30件が
通常工程である。現在の`orchestrator.validateGateCapabilities`はsummary confirmation、pipeline、reviewer、
sensors、workspace evidence、dispatcher／per-unit、per-kind、CodeKBを未対応として拒否する。また
`validateGatePhaseState`はinitializationとconstructionのgateを拒否する。

固定catalogの3 initialization Stageはcapability検査だけなら通るが、phase検査で停止する。最初の通常工程
`intent-capture`は`summary_confirmation: required`、reviewer、`claim-sources`・`required-sections`・
`upstream-coverage`の3 sensorを宣言する。そのため、現在のGoは固定catalog上の通常工程を自然な入口から
1件も完走できない。

これはルール・知識の配信実装を止める理由ではない。配信構成は対応能力だけを持つ合成catalogで一連の
受領をE2E検証し、固定catalogでは実Stageの配信情報を組み立てた後、既存の未対応能力検査がfail-closedに
なることを確認できる。ただし、それを「固定catalogのworkflowを完走した」と表現してはならない。

summary confirmation、reviewer receipt、sensor実行を製品として実装する作業は、現在承認された
「ルール・知識をAIへ渡す」範囲から自動的に推測しない。知識を読めることを、工程完了や人間承認の証拠にしない。

## 後続順序

1. 保存済みstateのDepth reader。
2. 通常Stageのconsume／produce artifact path resolver。
3. 配信chunkを継続するtokenとfreshness検証。
4. rule、knowledge、stage手順、artifact pathをrun-stage配信へ構成する。
5. Codex受け手がrule全体を保持してから各pathを実際に読む経路と、一連のE2E検証。

この順序は依存関係を示すもので、各Issueの詳細APIと受け入れ条件は個別計画で固定する。

## 根拠

- 固定本家 `core/tools/aidlc-orchestrate.ts:2943-3060,3160-3210`
- 固定本家 `core/tools/aidlc-lib.ts:16479-16488`
- 固定本家 `core/tools/data/stage-graph.json`（33 Stage）
- Go `src/internal/orchestrator/gate.go:475-526`
- Go `src/internal/orchestrator/completion.go:63-132`
- [配置Markdownによる知識供給マイルストーン](../decisions/2026-09-04-file-based-knowledge-delivery.md)
- [自律実装の包括承認](../decisions/2026-09-05-context-delivery-autonomous-authorization.md)

