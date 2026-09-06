# `HUMAN_TURN`を固定本家相当の運用証拠として扱う

- 日付: 2026-09-06
- 状態: Accepted
- 置換対象: `2026-09-06-intent-capture-vertical-milestone.md`の「人間応答の信頼境界」のうち、
  Stageを実行するmodelが同じ利用者権限から`HUMAN_TURN`を技術的に生成できないとした保証
- 実装許可: 2026-09-06のユーザーによる、固定AI-DLC本家相当の「運用証拠」へ境界を狭めて
  実装を継続する直接承認

## 背景

Issue #118の初回実装は、Codexの`UserPromptSubmit` hookから公開`aidlc` binaryの内部commandを呼び、
active intentのaudit ledgerへ`HUMAN_TURN`を記録した。独立reviewでは、同じ利用者権限でshellを実行できる
modelや別processもその内部commandを直接実行できるため、「Codex hostだけが発行できる」という認証境界に
なっていないと指摘された。

現行Codex Hooksの公開契約は`session_id`、`turn_id`、`hook_event_name`等をstdinで渡すが、hook発行者を
検証する署名、秘密capability、認証済みIPCを提供しない。これらの値、環境変数、project内の秘密、親process
検査は、同じ利用者権限のprocessに対する発行者証明にならない。厳密な偽造防止には、Codex側の署名済み
attestationまたはmodelから到達できないOS／MDM管理の外部trust brokerが必要であり、現在のproject配布範囲を
超える。

固定AI-DLC `2.6.123`も、audit ledgerを改ざん不能な認証基盤とは扱わない。`HUMAN_TURN`はsupportedな
prompt-submitまたはanswered-widget seamが発火した時系列上のpresence evidenceであり、応答本文の人間著者性を
証明しない。無人driverは協調的に`AIDLC_UNATTENDED=1`を設定してreceipt発行を止める。

## 決定

`HUMAN_TURN`は、固定本家と同じく**運用上の時系列presence receipt**として扱う。人間が認証済みであること、
応答本文を人間だけが作成したこと、同じ利用者権限を持つ悪意あるprocessから改ざん不能であることは保証しない。

Go版のsupported contractでは、次の境界を維持する。

- productionの正規emit pathはCodex `UserPromptSubmit` hookが所有する内部commandとする。
- 公開`aidlc report`、`--user-input`、公開audit append APIは`HUMAN_TURN`をmintしない。
- 承認・拒否は、reportに渡す厳密な選択値と、最後のworkflow resolutionより後にあるfreshな
  `HUMAN_TURN`の両方を要求する。
- 内部hook commandは公開helpやStage model向けprotocolへ提示せず、modelからの直接実行をsupportedな
  操作にしない。ただしcommand名を隠すことを認証・認可とは扱わない。
- `AIDLC_UNATTENDED=1`ではauthority receiptを発行しない。この環境変数は協調的driverの宣言であり、
  同じ利用者権限の攻撃者に対する防御とは扱わない。
- hook payload本文はauthorityに使わない。固定本家との互換性のため、hook invocationを受けた後は空・旧形式・
  malformed payloadでもpresenceを記録できる一方、stdin読込みは上限を設けて無期限・無制限入力を拒む。
- hookがworkflow状態を観測してからappendするまでにresolution、Stage、identity、audit generationが変わった場合、
  同一record lock内の再検証で古いturnを拒み、hook自体は人間promptを妨げないようno-opで終了する。
- Codex `PreToolUse`等のguardを加える場合もdefense-in-depthの誤操作防止と位置づけ、完全なenforcement boundaryとは
  説明しない。

この運用証拠は、同じ利用者権限のagentとproject fileを信頼するlocal development workflow向けである。
相互に信頼しないtenant、悪意あるagent、法的署名、金銭・本番権限など不可逆で高影響な承認の認証根としては
使用しない。その用途が必要になった場合は、外部trust brokerまたはCodexの検証可能なattestationを別計画で
承認する。

## 固定本家との意図的な差分

### 本家の挙動

固定AI-DLC `2.6.123`のCodex配布は、`UserPromptSubmit`に加えて`PostToolUse`の
`request_user_input`回答でも同じrecorderを起動する。どちらも認証済み著者証明ではなく、supported seamが
発火したpresence receiptである。

### 採用する挙動

最初のGo `intent-capture` milestoneでは、production emit sourceを`UserPromptSubmit`だけに限定する。
`request_user_input`回答だけでは`HUMAN_TURN`をmintせず、人間は通常promptとして応答する。

### 変更理由

公開APIと人間応答seamを最小範囲に保つため、ユーザーが`UserPromptSubmit`だけを直接承認した。最初の通常Stageは
通常promptの往復で完走でき、answered-widget固有のreceipt契約はまだ公開する必要がない。

### 利用者・互換性への影響

固定本家で`request_user_input` widgetの回答だけに依存する利用手順は、最初のGo milestoneではそのまま使えない。
人間が通常promptを送信すれば`UserPromptSubmit` receiptが作られる。この差分は将来widget seamを追加するときに
互換性gateとして再評価する。

## 実装とreviewへの影響

独立reviewの「内部commandを同じ権限から実行できる」という事実は維持するが、この事実だけをblockingな
認証bypassとは扱わない。reviewはsupported contractについて、公開report／auditからreceiptが生成されないこと、
hookなし・無人実行・古い観測ではgateがfail closedになること、hook処理自体はpromptを妨げないことを確認する。

Issue #118の残るblocking findingであるpayload互換、観測とappend間のTOCTOU、workflow errorとlock解放errorの
誤分類、catalog special fileのblocking、fresh sandbox hook E2Eは、この変更後の境界内で修正する。

## 根拠

- `docs/ram/decisions/2026-09-06-intent-capture-vertical-milestone.md`
- `docs/配布_ai-dlc/.codex/hooks.json`
- `docs/実装_aidlc-workflows/core/hooks/aidlc-record-human-turn.ts`
- `docs/実装_aidlc-workflows/core/knowledge/aidlc-shared/audit-format.md`
- OpenAI Docs `Hooks`: `https://learn.chatgpt.com/docs/hooks`
