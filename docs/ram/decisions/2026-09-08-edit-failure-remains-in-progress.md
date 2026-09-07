# 編集失敗後も同じIntentで作業を続ける

状態: Accepted。Issue #128、M1の補足計画。ユーザーの直接承認。

## 合意と背景

ユーザーは編集失敗も作業途中と捉え、AIが考え直して完了まで進めるよう提案した。
「作業の未完了は維持し、終了した編集ツールの実行中表示を解除する。修正・検証・同じKDRへの保存まで
続け、利用者に毎回の復旧操作を求めない」と説明し、「はい。オーケー」と承認された。
[前の確認待ち記録](../research/2026-09-08-minimal-live-failure-recovery-gate.md)の対応方法をこの合意で確定する。

検証に使ったCodex CLI 0.153.4は一部の編集失敗でPostToolUseを発火しない。
AIが返された編集エラーと処理停止を確認し、既存の同一Intent復旧CLIを内部操作として実行する。
hookが終了通知を受信したと装うこと、思考だけで完了にすること、時間だけで実行中表示を消すことはしない。

## 修正計画と実装許可

M1と今回の直接承認を根拠として、次を同じIssueで実装する。
外部module、OS権限変更、工程state、全操作audit、独自snapshot/receiptは追加しない。

1. `src/internal/minimal/hook.go` の固定CLI例外を修正する。
   `session bind <id> --space <space> --session <session> --recover` が
   現在の会話・Space・Intentと一致するときだけ、残留した実行中表示があっても通す。
   既存Serviceは実行中表示を解除し、未記録を保持してKDRと必須Ruleを読み直す。
   通常bind、別session/Space/Intent、混在shell、一般操作の制限は維持する。
2. 配置skillとhookの診断へ正確な復旧コマンドと適用条件を記す。
   AIがエラーを確認して再試行し、動作中のBashはpoll完了を待つ。
   最後の一般操作が終わってからKDRを更新し、その後に追加確認した場合は結果を再記録する。
   Stopの補完診断は内部の未記録状態と必要な操作を具体的に伝え、完了条件や一回再入の契約を緩めない。
3. `src/internal/minimal/session_test.go` と `src/cmd/aidlc/minimal_journey*_test.go` に
   終端Postがない編集失敗→AIの明示復旧→再試行→同じKDR更新の証拠を追加する。
   実機の試験用環境で意図したpatch失敗を起こし、利用者の復旧介入なしに続けられることを検証する。
   関係するinstall test、`docs/development.md`、実装証拠RAMも対象に含む。

単独writerは既存go_tdd_implementer。親は計画・RAM索引・Issue・review・final・PR・mergeを担当する。
既存のユーザー編集AGENTS.mdとローカル参照資料は保全する。

## 順序付き検証

作業単位 `m1-edit-failure-recovery`、verification_mode=loop。
同一session/Intentのrecover拒否を回帰testのREDで確認し、最小修正でGREENにする。
次に他の操作の拒否、未記録保持、Rule再読込、復旧後の更新を検査する。
最後に配置手順と実機判定器を補い、不足した復旧証拠では失敗することを確認する。
targetedは `go test -count=1 ./src/internal/minimal -run '^Test(Session|Hook|Rules)'` と
影響するinstall test、`go test -tags=integration -count=1 ./src/cmd/aidlc -run '^TestMinimalJourney'`。
本物のモデルは親のfinalでのみ起動する。

独立review後、全package test、race、vet、format check、tidy差分、既存CI統合試験、6構成buildを再実行する。
通常一周と質問・故障・復旧・再開の二つのliveも再実行し、同じKDRへの記録とclean終了を確認する。
失敗結果は成功扱いせず原因を修正する。PRの実際のchecks成功後に通常mergeしIssueを閉じる。

本家AI-DLC 2.6.123からOKF KDR方式へ変更する既承認の差分は維持する。
今回の運用は、Codexの欠けた失敗通知を新設したと主張せず、AIが失敗を確認して継続する最小構成である。
