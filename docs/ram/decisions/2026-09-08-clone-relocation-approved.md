# 配置移転とUnit再割当の実装を承認

状態: Accepted。ユーザーは[対応案](../../design/clone-relocation-proposal.md)への確認に「はい」と回答した。
既存資産を保持したhook/Skillの参照先更新、旧処理終了を確認した後の新担当への明示再割当を採用する。
同じIntent/Unitと成果を保持し、旧runtimeのGit共有や遠隔processの推測停止は行わない。

[実装計画](../../design/clone-relocation-implementation-plan.md)に保存順序・再試行・複数Unitの確認待ち・
CLI/help・実CLI/live検証を具体化した。新APIはこの計画のrelocate/reassign。新工程stateや全操作auditは追加しない。
既存原稿との判別を保つためSKILL原稿は変更せず、手順はWORKFLOWとhelpへ追記する。
Codexの新hook pathに対するtrustは利用者側の設定であり、自動変更せず必要な確認を案内する。

固定本家2.6.123のharness/codex/manifest.ts:21-60、emit.ts:199-226/401-470、
hooks/aidlc-codex-adapter.ts:194-208、guide/harnesses/codex-cli.md:37-95を確認した。
配置path・新root基準は整合。同名の移転・再割当契約は確認されず、承認済みGo四段階製品の機能として扱う。
本家の通常copy再配置を、利用者編集資産保護のrelocateと同等とは説明しない。
