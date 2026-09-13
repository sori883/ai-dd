# 外部由来skillに製品の接頭辞を付けない

2026-09-13。ユーザーは「ai-dlc等の独自のskillsじゃないものは、ai-dlcという接頭語を付けないで。
人のものだから」と修正を依頼し、ライセンスの許容とは別に作者への帰属を明確にすることを求めた。
この直接依頼を[命名・帰属修正計画](../../design/upstream-skill-names-plan.md)の実装根拠とする。
対応は[Issue #198](https://github.com/sori883/ai-dd/issues/198)、分類はユーザーリクエスト。

11工程skillの`aidlc-`を外し、原典の名前を使う。外部OKF手順を翻案した`aidlc-okf`も、
原典frontmatterにある`okf-agent-memory`へ変更する。13個の翻案skillは本文冒頭で原典・作者・翻案を示す。
元のLICENSEと固定commitを保持し、OKFに不足していた出典文書・原典LICENSEも配布する。
独自の進行用`aidlc`、操作用`aidlc-cli`、任意の`aidlc-github`、製品の担当名は維持する。
`natural-japanese-go`は既に接頭辞がなく、Go移植という変更を示す現名を維持する。

[工程skill導入](2026-09-12-stage-skills-natural-japanese-go-approved.md)と
[OKF skill導入](2026-09-12-aidlc-okf-skill-approved.md)の命名を置換する。
[共通化](2026-09-13-common-skills-codex-approved.md)の構造は維持するが、工程内のskill参照名を直すため、
該当本文と定義hashの不変条件だけを今回の修正に合わせる。旧記録・旧計画は履歴として保持する。
新規配置で検証し、既設fileや保存stateの更新・移行を実装しない。Go module・権限・承認の変更はない。
