# 3環境対応に向けたhook接続の前提確認

2026-09-12。対象は[製品対応の直接依頼](../decisions/2026-09-12-three-host-product-implementation-approved.md)と[実装計画](../../design/three-host-distribution-plan.md)。本記録は接続情報の確認であり、Claude・Copilotで製品全工程が完走した証拠ではない。

## 固定した環境と試験範囲

Codex CLI 0.153.4、Claude Code 2.1.238、VS Code 1.135.0（08d4889f9ec4a1685d257b9b95de036c8e1ce1e5）、同梱Copilot Chat 0.63.0を確認した。最低対応版はまだ確定していない。外部moduleやアプリは導入していない。

専用資材は `/Users/const/sori883/ai-dd-validation/three-hosts-20260912/`。Go標準ライブラリの観測用hook、架空のseed.md、読取り専用の試験agentを配置した。通常のpermission modeで実行し、利用者の既存projectやユーザー全体設定を変更しなかった。生の試験イベントは同folderのeventsへ分離し、RAMへ会話全文や認証情報は保存しない。

## Claudeで実測したこと

| 操作 | 実測結果 |
| --- | --- |
| SessionStart | session_id、cwd、source、transcript_path。prompt_idとpermission_modeはこのイベントには無かった |
| UserPromptSubmit | session_id、prompt_id、実際のpromptを取得。再開後の次のユーザー入力では別のprompt_idになった |
| Read成功 | Pre/Postが同じsession_id・prompt_id・tool_use_idを渡した |
| 指定Bash拒否 | PreToolUseのpermissionDecision=denyで、無害な試験commandが実行拒否された。対応する成功Postは発生しなかった |
| Agentによる子起動 | input.subagent_typeとrun_in_background=falseを確認。子Startと子Readにも親と同じsession_id・prompt_id、子のagent_id・agent_typeが付いた |
| 子終了と親応答 | SubagentStop.agent_idと、親Agent Postのtool_response.agentIdが一致。親Postのstatusはcompleted |
| Read失敗 | 存在しない試験fileに対するPostToolUseFailureが同じtool_use_idで届いた。error本文も存在した |
| 同じ子への追加依頼 | SendMessageのtoで既存agent IDを指定。PostのresumedAgentIdが同じIDになり、子のReadと応答を確認。新しいAgent起動は行わなかった |
| 自動通知の入力イベント | backgroundで再開した子の完了通知も、別prompt_idを持つUserPromptSubmitとして届いた。promptはtask-notification要素で、構造化した送信者origin欄は無かった |

初回のCLI起動は可変長の--tools引数にpromptが吸収され、入力不足で失敗した。`--`でprompt境界を明示して再実行した。これはhost機能の故障とは扱わない。また最初の再開試験ではSendMessageをtoolsへ含めていなかった。追加後に実際の再開が成功したため、最初の自己報告を『Claudeは再開不可』の根拠にしない。

現行CLIでは追加依頼はAgent.resumeではなくSendMessage.toを使った。古いSDKのschema例を固定CLIの実測より優先しない。[公式subagentの再開手順](https://code.claude.com/docs/en/sub-agents#resume-subagents)もSendMessageによる再開を説明している。ツール許可とAI-DLCの成果承認は別物であり、今回の試験で製品の人間承認を代行したわけではない。

[公式hook資料](https://code.claude.com/docs/en/hooks)にあるイベントと照合した。ただし、background初回起動・強制中断・通知欠落・OS別挙動・製品の承認と進捗連携はまだ実機確認していない。結果提出やStopだけでworker枠を解放する根拠にはしない。

**UserPromptSubmitというイベント名とprompt_idだけでは、人間の回答だと断定できない。** 子の自動通知を成果承認の証拠として取り込まない区別が必要であり、その製品契約を解決するまではD2の承認接続を実装しない。観測したXML形式だけで将来の全自動入力を判別できるとも断定しない。

## VS Codeで確認したことと未実測

画面操作ツールはmacOSのAccessibility・Screen Recording権限がpendingと返し、2回の接続でUI状態を取得できなかった。実際のCopilot会話やhookを動かせたとは扱わない。迂回手段で権限を変更していない。

固定版に同梱された公開製品コードの必要な箇所を読み取り、公式資料と照合した。

- `extensions/copilot/dist/extension.js` のChatHookServiceは共通timestamp・session_idとイベント別情報を渡す。Pre/Postはtool_name・tool_input・tool_use_idを含む。
- 子のSubagentStartにはagent_id・agent_typeが付く一方、子の通常PreToolUseには子conversationのsession_idだけが付き、親sessionやagent_idは含まれない。
- hook stdinに親runSubagentと子conversationを直接結ぶ対応値は見つからなかった。内部traceで扱う親子情報を製品の公開hook契約だと見なさない。
- Claude設定もVS Codeが読むため、同じprojectへの配置では二重実行の扱いが必要。単にsession IDやtool名だけで発生元を推測しない。

根拠: [VS Code hook](https://code.visualstudio.com/docs/agent-customization/hooks)、[hook入力仕様](https://code.visualstudio.com/docs/agents/reference/hooks-reference)、[subagents](https://code.visualstudio.com/docs/agents/run/subagents)。本家2.6.123のCopilot emitが記載した過去の試験結果を、このGo製品の実測へ流用しない。

## 確認中の設計選択

ユーザーへ次の2点を質問した。回答を得るまでは採用済みにしない。

1. Copilotでは子が起動直後にCLIへ割当IDと自分の会話IDを登録し、未登録の子の実装を許可しない。起動自体はmainのnative機能のままにする案。
2. 同projectに同居する場合、VS Codeではproject内Claude設定のhook読込みを止め専用hookを使う案。同projectの他のClaude用hookもVS Codeから実行されなくなる影響を明示した。代案はprojectごとにAI環境を1種類だけ配置すること。

この2点に依存しない配布分離D1はIssue #183で進める。共通のsession・予約保存形式、追加導入、環境切替、初期化Sensorの契約は、回答と実測に沿って後続計画へ具体化する。
