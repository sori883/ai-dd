# 本家のClaude承認方式を確認し、専用コマンドの案を改訂する

2026-09-12。調査・計画改訂の記録。ユーザーは、Claudeの自動通知が入力イベントになる問題の説明後、本家AI-DLCを調べて修正するよう依頼した。製品対応そのものの実装依頼は受領済みだが、新しい承認条件の採用とD2全体の接続契約を確定した記録ではない。

## 結論と前案の訂正

本家のClaudeでは、メインAIが標準の質問画面 `AskUserQuestion` を開き、利用者が `Approve`（承認）または `Request Changes`（修正依頼）を選ぶ。その回答をAIがCLIで記録する。利用者が専用の承認コマンドを手入力する方式ではない。

一方、本家の入力記録hookも、人間と自動通知の発信元を認証してはいない。「本家をコピーすれば自動通知を完全に除外できる」とは結論できない。

[専用コマンドの限定実測](2026-09-12-claude-user-command-approval-probe.md)は、候補の動作を確認したものだった。質問画面を調べ切る前に専用コマンドを推奨した点を訂正し、その案と採用質問を取り下げる。観測済みのイベントや失敗通知の記録は削除しない。

## 参照版と確認範囲

基準はリポジトリの技術分析と一致する固定snapshot **AI-DLC 2.6.123**。`core/tools/aidlc-version.ts` の値を確認した。作業checkoutにはignored snapshotがないため、元checkoutの `/Users/const/sori883/ai-dd/docs/実装_aidlc-workflows/` を読み取り専用で参照した。Go製品の作業先は `/Users/const/sori883/ai-dd-naming`。元checkoutの未commit資料は変更していない。

オンラインのmainも補足確認した。取得した[version定義](https://raw.githubusercontent.com/awslabs/aidlc-workflows/main/core/tools/aidlc-version.ts)は **2.7.1** であり、固定snapshotとは区別する。[質問描画手順](https://raw.githubusercontent.com/awslabs/aidlc-workflows/main/harness/claude/skills/aidlc/question-rendering.md)と[公式hook解説](https://awslabs.github.io/aidlc-workflows/reference/06-hooks-and-tools/)にも質問ツールと入力記録の説明がある。mainの取得結果を固定commitや本製品の参照版変更とは扱わない。

この調査では本家の承認手順、hook、判定関数、関連テストのコードを読んだ。本家のテスト実行、今回の方式のClaude実機実験、製品コード・設定・Issue・PRの変更は行っていない。

## 本家で確認した仕組み

以下のパスと行は固定2.6.123のsnapshot内を示す。

| 層 | 確認した内容 | 根拠 |
| --- | --- | --- |
| 人間への提示 | 構造化した質問をClaudeのAskUserQuestionへ変換する。承認はApprove／Request Changesで、選択値をそのまま記録する | `harness/claude/skills/aidlc/question-rendering.md:1,46,151`、`harness/claude/skills/aidlc/SKILL.md:104` |
| 工程の承認 | 成果と待ち状態を報告し、回答を待つ。Approveならapproved、Request ChangesならrejectedとしてCLIへ報告する。選択外は対話後に再提示する | `core/aidlc-common/protocols/stage-protocol.md` の承認手順、`core/tools/aidlc-orchestrate.ts:7888` |
| hookの登録 | UserPromptSubmitとPostToolUseのAskUserQuestionに、同じ入力記録hookを登録する | `harness/claude/settings.json:80,113` |
| 入力の記録 | 条件を満たせばHUMAN_TURNを記録する。promptやtool_responseから応答文を取り出す。自動通知の発信元を分類する検査はない | `core/hooks/aidlc-record-human-turn.ts:27,101` |
| 再利用防止 | 原則として、直前の承認や質問回答などの解決より後のHUMAN_TURNを必要とする。同じ入力で後続gateを連続通過させない | `core/tools/aidlc-lib.ts:6180`、`tests/unit/t188-human-presence-gate.test.ts:777` |
| 計画承認の追加照合 | sessionごとの提示済み選択肢と応答を照合し、応答hashを保存する。質問Postのanswersを扱うテストもある | `core/tools/aidlc-testing-posture.ts:1391`、`tests/unit/t328-plan-approval-runtime-authority.test.ts:253`、`tests/unit/t265-plan-approval-guard.test.ts:907` |

HUMAN_TURNは「人間の入力があったと扱うための記録」であり、本人確認ではない。無人試験を行う側が `AIDLC_UNATTENDED=1` を設定すると、この記録を抑止する。本家自身が、通常のUserPromptSubmitには入力者を区別する信号がないことをコメントで説明する。`humanTurnMintAllowed()` はこの環境変数を確認し、通知本文の構造などを検査しない（`core/tools/aidlc-lib.ts:16554`）。この自己申告を通常の自動通知の判別器として利用できない。

本家には `isMeta` 等を使う会話分類もあるが、Stop hookが会話終了を許すための処理であり、承認の入力記録へ接続されていない（`core/hooks/aidlc-continue-workflow.ts:739,786`）。存在するだけで今回の問題が解決済みとはしない。

本家の汎用承認判定も、引用が人間由来だと証明できない旨を明記している（`core/tools/aidlc-lib.ts:6340`、`core/tools/aidlc-state.ts:4773`）。承認記録がないときの互換的な通過やautonomous modeなど、現製品の承認契約を弱める挙動は流用しない。

## 現在のGo製品との関係

現在の `src/internal/app/hook.go` はCodexのUserPromptSubmitを `flow.CaptureApproval` へ渡す。`src/internal/flow/approval.go` は、その回答を受けた時点で待っている承認ID、会話、回答ID、引用を後続CLIの指定と照合する。イベントを受けただけで自動的に承認済みにはならず、AIによるCLI登録が別に必要である。

Claudeの通常Submitを同じ保存入口へ直結すると、自動通知も承認の引用候補になり得る。これは「通知だけで必ず承認される」という不具合を実証したものではないが、回答の取得経路としては区別が不足する。Claude向けの新規接続で対処する必要がある。

## 改訂案と本家との差分

[D2計画](../../design/claude-code-connection-plan.md)を次の一案に改訂した。

- 利用者は本家と同じ質問画面で承認または修正依頼を選ぶ。本文・説明は日本語で、承認IDの転記や専用コマンド入力を要求しない。
- 質問前に対象の承認IDと版、会話、tool呼出しIDを結び付け、質問後の回答で同じ対応を照合する。選択値とCLIの判断が逆なら拒否する。
- **本家との差分:** Claudeの承認証拠は、対象に対応する質問のPostToolUse回答だけから作る。通常Submitからは作らない。理由は観測した自動通知を承認入口へ入れないためであり、利用者は自由文の「はい」だけでは承認を完了できなくなる。この利用条件は実装前に採用確認する。
- 計画と成果は一つずつ別の承認質問にする。自由記述・取消・無回答は承認待ちを保持する。Codexの既存操作、現在の工程とSensor・review、CLIによる判断記録を維持する。
- 全操作auditや新しい工程stateは導入せず、質問と回答の照合に必要な一時記録だけをD2の保存契約に含める。時間経過、通知、tool終了を承認の代わりにしない。

本家の画面と、Go製品の承認対象の照合を組み合わせる提案である。質問の回答を取得した事実が、人間本人の認証や他のhookによる変更まで禁止する保証になるとは説明しない。

## 固定Claudeで残る検証と承認状態

Context7でClaude公式資料を確認した。[CLI hook仕様](https://code.claude.com/docs/en/hooks#pretooluse-decision-control)では、非対話の `-p` で質問の回答をhookのupdatedInputに渡す経路がある。これを使っても人間が質問画面に回答した実験にはならないため、本提案の受入試験では自動回答で代用しない。[SDKの質問出力型](https://code.claude.com/docs/en/agent-sdk/typescript)も参照したが、CLI hookの同じJSONを実測した証拠とは区別する。

固定Claude Code 2.1.238の対話起動で、選択・自由記述・取消、Pre/Postの対応ID、質問中の自動通知、tool失敗と再送を確認する。入口が成立しない場合は方式を再検討し、専用コマンドや本文推測へ黙って切り替えない。

今回のユーザー依頼は本家調査と計画改訂の根拠である。回答経路を限定する差分の採用、初回導入範囲、実機で未確認の入力・保存契約まで解決した承認とは扱わない。旧専用コマンドの質問は取り下げ、新案の利用条件を説明して確認する。初回導入範囲の既存の質問は別の未決事項として保持する。

改訂後、別の計画担当が読み取り専用で確認し、重大な矛盾や修正必須箇所はなかった。変更文書のローカルリンクと差分の空白検査を確認した。これは文書の検証であり、Goの独立コードreviewや製品実機試験の成功を意味しない。
