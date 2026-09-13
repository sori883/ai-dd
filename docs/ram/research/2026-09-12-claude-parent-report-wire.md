# 固定Claudeの子からメインへの報告と通知順序

2026-09-12。Issue #185のD2アダプター計画に必要なwire形式を、隔離した試験projectで確認した。対象はClaude Code 2.1.238の固定実行ファイル。通常のprint起動であり、対話的な信頼確認や製品hookによる拒否判定の成功証拠ではない。

子の報告先は `SendMessage` の `to: "main"` である。固定実行ファイルの内蔵説明はbackground子からの送信先としてmainを列挙している。`parent` や `team-lead` を通常のcustom agentの親宛先として採用する根拠はない。今回の実測では同期・backgroundの双方でmainへの配送が成功した。固定版で確認した範囲に限り、他の版やteam機能まで同一だとは扱わない。

## 実測

試験rootは `/Users/const/sori883/ai-dd-validation/three-hosts-20260912/claude-project`。ReadとSendMessageだけを持つ試験担当が架空のmarkerを自身のメインへ送った。hookのPre/Postとメインの実際の受信記録を照合し、担当の自己申告だけでは判定していない。両試験はexit 0で終了し、子の完了も確認した。

| 実行方式 | session | child ID | SendMessage tool ID |
| --- | --- | --- | --- |
| 同期 | `73542c71-85d9-4199-9c6c-514b67010a51` | `ab43bfcce7cbd27b3` | `toolu_014i7iJR3pSRGmKgqxuetRKS` |
| background | `0a7da3c2-8f21-4417-928a-ee260b93b3b7` | `a39da2c3165688f19` | `toolu_01WLEsv3qqpuyd4PBy5pZg3c` |

同期ではRead→SendMessage成功→Stop→親Agent Postを確認した。メインのpeer attachmentは送信child IDとmarkerに一致した。backgroundでは親Agent PostがSubagentStartより先に届いた。子のメッセージは `origin.kind: peer`、一致する `senderTaskId`、`isMeta: true` の記録として配送され、その通知が新しいUserPromptSubmitを発生させた。後続SubagentStopのprompt IDも新しい入力回の値だった。

したがって通常Submitを人間の承認とは扱わず、Stopのprompt IDが起動時の値と一致することも必須にしない。予約はStopや結果提出だけで解放しない。同期子の最終回答は通常経路で返るので、SendMessageを必須作業にはしない。

今回のAgent入力はnameを省略していたため、name保持の根拠には使わない。nameの観測は[先行試験](2026-09-12-claude-named-agent-adapter-contract.md)を参照する。

## 証拠と適用

固定実行ファイルは `/Users/const/.local/share/claude/versions/2.1.238`。raw hook記録、出力、実受信記録のpathとSHA-256は試験用 `product-g0/parent-report-wire-observation.json` に保存した。製品G0では同じ宛先をアダプターが解釈し、結合済みの子・親会話・現在Intent/工程/worker予約を照合できることを別に確認する。

現在の[公式SDK資料](https://code.claude.com/docs/en/agent-sdk/typescript)と[会話間メッセージ資料](https://code.claude.com/docs/en/cross-session-messaging)も確認したが、固定版のcustom child→main宛先とhook順序の根拠は今回の固定実行ファイルと実測である。
