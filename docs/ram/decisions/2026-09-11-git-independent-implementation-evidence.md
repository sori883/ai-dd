# Git不要化と集合SHAの実装証拠

日付: 2026-09-11。Issue #171。直接承認した[実装計画](../../design/git-independent-workflow-plan.md)に従う。G0の通常Codex確認は[別記録](2026-09-11-git-independent-g0-codex.md)を参照する。

## 実装した契約

製品の管理元をGitから探す方式を、明示rootと配置済みstage-graphの探索へ変更した。検証はIntentの `verification_paths` をまとめたSHA-256で行い、必要なUnitだけ範囲を細分化する。ファイル数・容量、範囲外参照、symlink、読込み中の変更を検査し、管理記録をコードのSHAへ混ぜない。

Unitの提出は担当run・現在内容・実測結果を照合し、反映は管理元の内容と提出SHAを照合する。同じrootの順次割当と別会話の独立レビューを許可した。同一・親子rootのworker予約は競合し、結果提出や終了通知だけでは解放しない。後続Unitによる変更で過去のintegratedを戻さず、工程終了時には最新全体SHAの検証を要求する。

flow schema 6、assignment schema 2を使用し、旧commit field・互換読込み・移行を設けない。利用者の旧ファイルは削除しない。新CLI、help、製品Skill・担当・工程、配置手順と継続検証を更新した。

## TDDと親の作業単位末尾確認

単独writerがwork unit `git-independent-workflow` の9項目を順に実装した。対象は管理元探索、集合SHA、新CLIとschema、通常rootの担当管理、検証結果、Unit、Sensorと承認、配布、一周fixtureである。各項目のrunnable RED（exit 1）とGREEN（exit 0）を保存し、親も計画にある9コマンドを一度再実行してすべて成功した。

証拠の保存先は `/Users/const/sori883/ai-dd-validation/git-independent-171/tdd/` と同階層の `parent-boundary/`。`work-unit-report.md` と各JSONにコマンド・終了コード、各logに実出力を持つ。S2初回のtest helper不良はINVALID_REDとして区別し、S3の追加schema回帰とS7の既存hookで初回から通る追加coverageも別記録とした。compile failureや未実行テストをREDと数えない。

既存テストの廃止されたGit祖先・別worktree条件は内容照合・通常rootへ置き換え、停止確認と保存復旧を維持した。公開helpの別root必須という残存説明も親の確認で修復し、追加回帰のRED→GREENと親の再実行を確認した。全package・race・vet・配布・実Codex検証をloopの成功と混同しない。

## 最終確認の扱い

この記録の作成時点では独立reviewとread-only finalが残る。固定commitへの独立review、全体検証、実Codexでの同root worker・競合拒否・旧結果失効・同root reviewer・承認待ち拒否、およびPRの全checksを経てmergeする。生証拠と最終結果はIssue・PRへ追記する。実機前提の合成承認・レビューは、実際のユーザー回答や実AIレビューの証拠には数えない。

本家固定2.6.123で確認したGit worktree・Git ref方式から通常フォルダ・ローカル予約・集合SHAへ変更する点は承認済み。Gitの管理単位に左右されず順次作業するためであり、Git履歴の包含や担当外変更の網羅的な検査をSHAが代行するとは説明しない。本家全体や最新upstreamとの一致は未確認である。
