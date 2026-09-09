# 共通操作の索引

入口で指定した実行ファイルを `A`、Intentを `ID`、Spaceを `SPACE` と記す。
`A intent procedure ID --space SPACE` が現在段階の手順全文、担当、文書参照と遷移候補を返す。
各turnのbindが返す必須Ruleを読み、段階変更・再開後は現在手順を取り直す。
現在のrevisionは `A intent show ID --space SPACE` で確認する。

- 計画と型・JSON例: `A intent configure --help`
- 検査・開始: `A intent check --help`、`A intent begin --help`
- 独立review: `A intent review --help`
- Unit割当・回収: `A unit claim --help`、`A unit result --help`、`A unit integrate --help`
- 質問待ち・再開・差戻し: `A intent wait --help`、`A intent resume --help`、`A intent reopen --help`
- 本文草稿からKnowledge保存: `A memory create --help`、`A memory update --help`
- clone移転: `A install codex --help`、`A unit reassign --help`

定義JSONと全段階MDのpath・本文bytesはIntent作成時に固定される。変更時は元版へ復元するか新Intentを作る。
既設配置を上書き更新しない。欠落時に旧手順や内包版を代用しない。
差戻し理由はIntentのwork-log.mdへCLIが保存し、設計判断のADRとは区別する。
途中保存は成功ではない。同じexpect・元/先段階・理由で再試行する。logが変わった場合は元版へ復元する。
非同期toolは終端までpollし、失敗終了を確認した同session/Intent/Spaceだけsession bind --recoverを使う。

## 別cloneへ移ったとき

移転先のAI会話を開始する前に端末で `aidlc install codex --help` を確認し、
`aidlc install codex --relocate --project-dir NEW_ROOT --from-project-dir OLD_ROOT --from-binary OLD_BINARY`
を実行する。旧pathは配置済み参照と一致する絶対文字列を指定する。既存SKILL原稿の編集や版更新を兼ねない。
新しい `.codex/hooks.json` の絶対pathとCodex hook trustを利用者が確認する。認証設定を自動変更しない。
部分失敗のPaths/Pendingを読み、同じ引数で全件を再検査する。未知編集を推測で上書きしない。

旧workerの終了を確認し、`unit reassign --help` のJSONで同じIntent/Unitを新しい別worktree/sessionへ割り当てる。
runningなら既存pause/resumeでneeds_confirmationにする。`previous_run_stopped: true` は確認済みの場合だけ指定し、
CLI成功まで新workerを開始しない。runtimeはGit共有しない。保存途中は他のstate更新（configureや別Unit操作など）が拒否される。同じexpect/JSONで再試行し、
既にrunningならshowと現在assignmentで成功を確認する。新run_idと現在HEADで再テスト後result/integrateする。
レビューは新root/sessionへassignし、現在targetの独立reviewを受け直す。
