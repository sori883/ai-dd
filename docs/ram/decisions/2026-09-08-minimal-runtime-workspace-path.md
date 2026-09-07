# CLIとhookの一時ファイルをworktree内で共有する

- 日付: 2026-09-08
- 状態: Accepted implementation detail（M1直接承認内の保存先具体化）
- 対応Issue: #128

M0案は会話の一時情報とdraftをGit管理領域に置いていた。しかし通常のworkspace-write sandboxで
AIが実行するCLIやapply_patchから.git配下へ書くことを前提にすると、保護された管理領域への書込みが必要になる。
このため、権限を追加せず同じ作成・更新手順を実現する一時保存先へ改める。
固定版liveのturn_contextでもsandbox_policy=workspace-write、.git/.agents/.codexへのaccess=read、
worktreeへのaccess=writeを確認した。hook入力のpermission_modeだけでOS sandboxの強さを判断しない。

```text
<worktree>/aidlc/.runtime/
  .gitignore
  sessions/
  drafts/<session-id>.md
  locks/
```

runtime内の.gitignoreで配下をGit管理対象外にする。既存の利用者のroot .gitignoreを書き換える必要はない。
worktreeごとに独立し、Knowledge・KDR・Ruleの正本は引き続きSpaceのknowledge配下に置く。
session IDのpath要素検査、正規化root、symlink拒否、session lock→Space bundle lockの順序を維持する。
hookのdraft例外は、この会話の正確なdraft絶対path一件だけ。別pathや混在patchを免除しない。
runtime消失・破損を記録済みと解釈せず、同じIntentの再選択・現物確認から復旧する。

read-only project_plannerは、永続正本・利用者操作・権限・保証範囲を変えない一時保存の通常詳細として、
M1許可内で具体化できることを確認した。新しい工程state・全操作auditを追加する判断ではない。
Git管理領域を利用するという先行M0案の一時保存先だけを置換し、M1のCLI/patch/live試験もこのpathへ揃える。
固定Codexの本体一周で、CLIとapply_patchが実際に同じruntimeへ書けることを確認する。
