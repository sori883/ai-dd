# 製品のminimal名称を取り除く

日付: 2026-09-12。状態: 直接実装依頼を確認。

ユーザーは `src/core/minimal`、`src/harness/codex/minimal` のようなminimal名称を不要と指定した。
続いて、内部 `src/internal/minimal` や `__minimal-hook` も対象に含める確認へ
「はい。お願いしたいです。」と回答した。

[具体計画](../../design/product-naming-cleanup-plan.md)に従い、原稿をcore/Codex直下へ移し、
内部packageをapp、hook入口を `__hook` とする。型・呼出し・test・現行文書も一体で追従する。
これは製品の最小版という区分をなくす命名整理であり、機能の削除指示ではない。
旧33 Stageの包括承認を流用しない。

旧記録の後方互換・移行不要という[既存承認](2026-09-11-git-independent-implementation-approved.md)を維持し、
旧hook別名・自動移行は設けない。新binaryと新hook設定は組で扱い、既知の現行資材の移転と版更新を区別する。
既存利用者配置と元checkoutの未commit資料・agent編集は保全する。保存schemaとKnowledgeの場所は変えない。
過去の最小構成案・RAM・実測証拠は履歴として保持し、現行sourceだけを新名へ統一する。

開始時のmainはPR #172 merge後の `2cab52b52cf38338c682299e5e6a4e0788e491c4`。
Open Issue/PRは各0件、同じ目的のIssueは見つからなかった。
専用worktree `/Users/const/sori883/ai-dd-naming` で単独writer・独立review・read-only finalを行い、
通常のIssue/PR/checks/mergeまで進める。Go単一binary・外部module追加なし・native agent起動を維持する。

対応: [Issue #173](https://github.com/sori883/ai-dd/issues/173)、分類 `機能開発`。
