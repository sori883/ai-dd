# 最新実案件の初期化を成果承認待ちまで進めた

状態: Issue #163進行中。初期化の開始・終了Sensorと開発用の独立レビューがpass。
初期化成果の人間承認、製品native担当と通常hookの実測、後続段階、help実装、finalは未実施。
①完走前のため、②配布・更新、③利用者文書の実装にはまだ進んでいない。

## 対象と準備

- 元版: main `62429d4f644293093ef4e5ae2fcc3ec5a24b8c8b`（PR #162）。
- 準備commit: `b3bab02`（計画・今回の依頼RAM・索引だけ。製品code/moduleは元版から不変）。
- 開発branch: `codex/latest-workflow-pilot`。
- 専用調整root: `/Users/const/sori883/ai-dd-latest-pilot`。
- 独立review root: `/Users/const/sori883/ai-dd-latest-pilot-review`。
- Space: `workflow-pilot`。
- Intent: `915a62dd6391fbab3e08a5707309d687`（check helpの文書検査案内を改善）。
- step: `s01 initialization`、現在revision: `4`、実行回status: `awaiting_approval`。
- 次回: `s02 discovery` はpending。

fresh installで6段階、5つの製品agent、aidlc/aidlc-cliの2skill、hookと初期OKF資産を配置した。
SpaceとIntentはCLIで作成。stateとreview登録もCLIだけで操作し、正本を直接編集していない。
元の `/Users/const/sori883/ai-dd` の未コミット資料は変更していない。
利用データと生成された利用者用設定は製品PRへ含めず、必要なfileだけを明示stageする。

## 実測とレビュー

| 確認 | 結果 |
| --- | --- |
| version、fresh install、space create、intent create | 成功 |
| 必須Ruleの全文とprocedure | 実CLIで取得。Ruleはプロジェクト用。outputsは空で可 |
| initialization-start | pass |
| begin | 入力path/hashを保存 |
| initialization-end | pass |
| 開発用独立レビュー | `/root/pilot_init_reviewer`、verification_mode=review、P0〜P3指摘なし |
| review accept | 実報告を受理して成果承認要求を生成 |
| 承認なしのfinish | exit 2、human result approval required。state bytes不変 |

独立reviewでは配布assetのhashと基準の埋込み原稿、hook絶対参照、Space Rule、
開始時の入力版、state、review targetを照合した。この担当は製品aidlc-reviewerのnative実測ではない。
最初にversionへ不要な --project-dir を付けた親の確認scriptはexit 2になった。
元の失敗記録を保持し、正しいversion操作の成功を別名で保存した。製品bugやTDDのREDには数えない。

実CLIのargv/stdout/stderr/exit code、配置manifest、承認要求は専用rootの
`aidlc/evidence/latest-pilot/` に保持する。

## ユーザーへ提示する成果承認

- request_id: `82fbc4a54de98997c7252ea3e2338224`
- target: `7320562fdda9e0ce40dba3bff1735cd0fec75bba1ff2df733f690a0e8e12cbb8`
- definition_hash: `6b9ea172726d5ff19309665e19bf23955f1034d97f0cf67966cd7ea3d59d405f`
- state SHA-256（revision4）: `f20fa6f2eb2a518be165981f898c8250f58eea6e4998956f5ffd4a1b3032e65d`

生成後にユーザーへ提示して回答を待つ。以前の作業開始依頼はこの要求への回答として利用しない。

## 次に必要な通常環境の確認

現会話は開発agentだけを読み込んでおり、新配置の製品5担当とhookを自動実行していない。
次は配置先を読み込んだ固定Codex CLI 0.153.4の通常会話を用い、メインAIのnative担当起動を観測する案。
aidlc CLIがagentを起動する形には変更しない。通常のhook信頼確認を通し、試験用bypassは使わない。
本会話の承認回答を中継する場合は、実際の原文と出典を保持し、受信した実hookのsession/turnを使用する。
Desktop画面操作の検証とは区別し、合成したhook入力を実通知と称さない。

対象hook: `.codex/hooks.json`

```text
command:
'/Users/const/sori883/ai-dd-latest-pilot/aidlc/evidence/latest-pilot/aidlc' __minimal-hook --project-dir '/Users/const/sori883/ai-dd-latest-pilot'
hook SHA-256: 22e36518903957612e044a2de1a9e2dbfa060e703d0de0f04f2b6a553059722a
binary SHA-256: 73470c02cfebb04e5e0227421352d8b7991c1ea4a635e7589e8c8c86c0b76466
```

対象はSessionStart/UserPromptSubmit/PreToolUse/PostToolUse/Stop。
管理registryはまだ作っていない。この専用rootでは製品workerを起動しておらず、回収待ちの製品成果もない。
ユーザーの初回管理開始の確認前に `human_confirmed:true` を自己申告しない。
通常trustと回答中継の実行経路が成立しなければ、必要な利用者操作を具体化して止める。

詳細な承認済み目的と実装範囲は [実施計画](../../design/latest-workflow-pilot-plan.md) と
[順次実施の直接依頼](2026-09-10-sequential-completion-request.md) を参照。
