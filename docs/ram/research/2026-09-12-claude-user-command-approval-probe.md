# Claude Codeの利用者専用コマンドと承認入力の確認

2026-09-12。Claude Code 2.1.238、macOSで確認。対象は[Codex・Claude Code対応](../../design/three-host-distribution-plan.md)の接続前提G0であり、製品の成果承認やD2実装の完了ではない。

後続の[本家承認方式の調査と改訂](2026-09-12-upstream-claude-approval-and-revised-plan.md)で、専用コマンドを推奨する案とその採用質問を取り下げた。本記録の限定実測結果は維持する。試験の成功は専用コマンドが必須であることを意味しない。現在の接続案はClaude標準の質問画面を使うものであり、以下の「確認中」は当時の状況である。

## 調べた理由

Claudeの子担当が返す自動通知も `UserPromptSubmit` として届くため、このイベントを無条件に人間の回答として保存できない。現在のGo製品は、hookが取得した回答を `flow.CaptureApproval` へ保存し、AIがCLIへ登録する判断の引用・承認対象と照合する。Claude向け接続では、その保存入口を区別する必要がある。

通常hookに構造化した送信者の種類は確認できなかった。Agent SDKの結果にあるorigin情報をCLI hookの入力に流用しない。本文のXML表現だけで全自動通知を判定する方式も採用していない。

## 公式仕様と実機確認

[UserPromptExpansion](https://code.claude.com/docs/en/hooks#userpromptexpansion)は、利用者が入力したコマンドが展開されるときのイベントとして説明されている。[skillの起動制御](https://code.claude.com/docs/en/skills#control-who-invokes-a-skill)では、`disable-model-invocation: true` によってAI自身のskill起動を禁止できる。

既存の標準ライブラリ製観測hookを再利用し、専用の `claude-approval-probe/.claude/skills/aidlc-approval-probe/SKILL.md` とhook設定を追加した。配置先は `/Users/const/sori883/ai-dd-validation/three-hosts-20260912/` の下だけである。元checkout・利用project・ユーザー全体設定・製品コードは変更していない。

| 試験 | 観測したこと |
| --- | --- |
| 入力として `/aidlc-approval-probe request-123 approve` を渡す | `UserPromptExpansion` を受信。`expansion_type=slash_command`、`command_name=aidlc-approval-probe`、`command_args=request-123 approve`、`command_source=projectSettings` が届いた |
| 入力イベントとの対応 | 同じsession_id・prompt_idの `UserPromptSubmit` が後に届いた。この試験ではExpansionがSubmitより先だった |
| AIからSkill toolで同名skillを起動する | tool_useと対応するtool_resultを試験会話記録で確認。`disable-model-invocation` によりerrorとなった。自己報告だけを拒否の証拠にしていない。Expansionは発生しなかった |
| background子とSendMessageによる再開 | native起動と同じ子への再開は成功。再開後の子はAPI側の拒否で失敗し、その失敗通知が別prompt_idのUserPromptSubmitとして届いた。Expansionは発生しなかった |

最後の試験は、子の再開後の正常完了を確認したものではない。API側の拒否を回避する再試行は行っていない。子の初回返却内容について親AIの説明はあったが、正常な完了通知がhookに届いた証拠としては扱わない。実測できた自動入力は失敗通知であり、全種類の通知を網羅したとは言わない。

試験は `claude -p`、通常のpermission mode、利用可能なtoolsを限定した条件で行った。明示的なpermission bypassは指定していないが、`-p` 自体がworkspace trust dialogを省くため、対話UIでの通常trust確認を実施した証拠にはならない。コマンド入力は外側の試験AIが架空のIDを渡したもので、人間による製品の成果承認ではない。

生のイベントと試験会話は専用環境に留める。RAMには秘密情報や会話全文を転記しない。

## 製品への接続候補と残る確認

Claudeだけ `/aidlc-approve <承認ID> approve` または `reject` を使う候補を提示した。AIが入力用の行を提示し、人間がチャットへ入力する。人間専用skillの正確なcommand名・source・args・会話と回答IDを検査し、現在待っている承認IDと一致するときだけ、その判断用の入力証拠を保存する。本文の `approve` という語があるだけでは受け付けない。

後続の通常UserPromptSubmitや自動通知で、この入力証拠を別の承認対象に付け替えない。計画承認と成果承認が同時に待っていても、コマンドに指定したIDだけを対象にする。入力のapprove/rejectと後続CLIのdecisionが一致しなければ拒否する。現在の引用の部分一致検査だけに任せない。再送・別ID・古い対象・保存失敗の検査も必要になる。判断と引用の記録は引き続きメインAIがCLIで行い、本人認証や全書込み経路の禁止を保証しない。

自然文の回答から専用コマンドへの変更は利用操作を変えるため、ユーザーへ方式を確認中。採用済みとは扱わない。採用後も製品hookでの拒否、対象照合、保存・再試行、対話UIのtrustと工程完走はD2・D3の受入条件に残る。

## 子担当の初回対応付け

先のG0で取得したSubagentStartにはsession_id・prompt_id・agent_id・agent_typeがあるが、親Agentのtool_use_idや構造化したnameは無かった。親PostToolUseにはtool_use_idとtool_response.agentIdの対応がある。ただし同期AgentのPostは子の実行後となるので、Postだけを待って初回編集を許可する設計は成立しない。

同じ親会話・回答回・役割について、まだ子IDに結び付いていない起動を一件だけ許可し、SubagentStartで対応を確定する案を計画候補とする。対応確定後も同じ役割を一人に制限する意味ではない。重複や不明な子は操作を拒否し、PostでもIDを照合する。単一pending候補だけで安全に結べるかは、順序違い・重複・通知欠落を含む実機と回帰検証が必要であり、確認前に製品の保証とはしない。
