# Git不要・集合SHA・同じ作業場所の順次作業を実装する承認

日付: 2026-09-11。状態: 具体計画全体を直接承認。Issue #171で実装を開始する。

ユーザーは[具体計画](../../design/git-independent-workflow-plan.md)に関して、旧記録の後方互換・移行を不要と指定した。その後、Unit成果の内容照合、同じフォルダでの順次作業、新しい保存形式の説明を受け、次のように回答した。

> 返しました。じゃあえっと実装してください。

この回答を、Gitに依存しない管理元探索、適切な範囲の集合SHA、テスト・Unit・レビュー・承認の対応、同じ作業場所の排他、新しい保存形式と配布手順を含む計画への直接の実装承認として扱う。Git不要化やSHAの単位、旧記録の互換性を再確認しない。旧33 Stageの包括承認や、未採用のshared-worktree-hook案は根拠にしない。

対応は [Issue #171](https://github.com/sori883/ai-dd/issues/171)「Gitに依存せず集合SHAと同じ作業場所で工程を完了できるようにする」。分類は `機能開発`。開始時のmainと作業HEADは `9360348cf0512d168b8604c85c24da322f186186`、Open Issue・PRは各0件で、同じ内容のIssueは見つからなかった。

作業場所は `/Users/const/sori883/ai-dd-worktree-hooks`。元の `/Users/const/sori883/ai-dd` にある未コミット資料と利用環境は変更しない。この開発用worktreeは元の変更を保全するための作業場所であり、製品の利用条件としてworktreeを要求するものではない。

## 実施順序と境界

1. G0として `/Users/const/sori883/ai-dd-validation/git-independent-171/` 配下の新規・非Git projectで、固定Codex CLI 0.153.4の通常trust、親子、hook入口への到達を確認する。現行製品のGit要求による拒否とCodexの読込不成立を区別する。
2. 前提成立後、単独の `go_tdd_implementer` がwork unit `git-independent-workflow` の9項目を `verification_mode=loop` で順に実装する。新しい型・APIは、testをrunnableにする最小の宣言と未実装返値のscaffoldを許可し、compile failureをREDとしない。
3. 親の作業単位末尾の確認、独立 `review`、read-only `final`、PR、対象checks、merge、Issue closeまで既定の開発ルールに従う。

Go単一バイナリ・標準ライブラリ・メインAIのnative agent起動・一人の実装writerを維持する。旧記録の互換読込み・移行・二重運用は追加しない。既存ファイルの削除や利用環境の一括初期化は行わない。担当の停止不明、保存失敗、実機の前提不成立を成功扱いしない。

本家固定2.6.123の確認済みGit worktree・Git ref方式から、通常フォルダ・ローカル予約・集合SHAへ変更する点と、その理由・影響は計画とIssueに記載した。Git履歴の包含や担当外変更の網羅的な機械検査をSHAだけで再現したとは説明しない。

この記録時点ではIssue作成と計画・承認記録の更新までで、製品コードの変更とG0実機検証はこれから行う。
