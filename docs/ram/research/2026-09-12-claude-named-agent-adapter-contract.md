# 固定Claudeの構造化nameとアダプター実装順序

2026-09-12。Claude Code 2.1.238の観測とD2への反映。製品実装の検証成功を示す記録ではない。

## 観測した条件

専用root `/Users/const/sori883/ai-dd-validation/three-hosts-20260912/claude-project` で、固定binary `/Users/const/.local/share/claude/versions/2.1.238` を使用した。通常のproject設定だけを指定し、MCPは空、toolはAgentとRead、最大4turnのprint mode。hookは受信JSONを記録する観測用であり、製品の担当予約・拒否処理ではない。print modeのため初回trust画面の検証とは扱わない。

親AgentのPre/Post両方に構造化された `name: aidlc_probe_named_001`、`subagent_type: aidlc-probe-reader`、`run_in_background: false` が残った。SubagentStartにnameや親tool IDはなく、子のagent_idとagent_typeが届いた。子ReadのPre/Post、SubagentStop、親Agent Postの順で、親Postのresponse.agentIdが子のIDと一致した。Readは専用seed文書のみで、結果は `THREE-HOSTS-SEED-20260912`。終了codeは0。

観測sessionは `3e371e75-82e8-4fc7-9984-0ec6842f5967`、promptは `e8aaafeb-8515-4f0e-9b4f-f3298f941a25`、親tool IDは `toolu_01KUUBXfJvh5SckUXtR28Gan`、子IDは `a46bc77c725ef0d6d`。ローカル証拠は同rootの `named-agent-output.json` と専用eventsの該当session。秘密情報や利用案件の本文はRAMに転載しない。

Context7経由の[公式Agent SDK TypeScript資料](https://code.claude.com/docs/en/agent-sdk/typescript)にもAgentInputの任意nameが記載される。ただしSDK型だけを実CLI hookの証拠にはせず、実際のPre/Post観測で補った。[hook資料](https://code.claude.com/docs/en/hooks)のSubagentStartはagent_id/typeを示す。nameがnativeの再開aliasになるかは確認しておらず、再開は既に観測したSendMessage.toのnative IDで扱う。

## 実装へ反映する契約

構造化nameを既存task_nameへ対応付け、自然文promptからrootや割当を推測しない。Startに親tool IDがないため、共通予約lock内で同じsession・turn・役割の未結合起動を一件に限定して結ぶ。結合後の全担当人数を一人に制限しない。回収済みの未結合候補を同turnで再利用せず、遅延Startの誤結合を防ぐ。

ユーザーの[新規配置・環境別アダプター指定](../decisions/2026-09-12-fresh-host-adapters-only.md)に従い、native JSONとID処理はharnessへ置く。共通sessionは既存6項目を維持し、アダプターがnamespaceとnative sessionからSHA-256のopaqueキーを生成する。assignmentはopaqueな子対応を扱い、native parserを持たない。

製品相当guardの別試作品を二重実装せず、観測済みwire契約から[D2製品アダプター](../../design/claude-code-connection-plan.md)をTDD実装し、その実コードの固定実機G0をreview/final/merge前の必須gateにする。read-only計画担当もこの順序と境界に重大な矛盾がないことを確認した。Start保存が初回子Preに間に合うこと、製品の担当拒否、再開、worker競合、質問復旧は引き続き実測が必要であり、本観測だけで成立済みとしない。
