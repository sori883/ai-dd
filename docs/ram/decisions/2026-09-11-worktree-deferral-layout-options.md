# worktree対応の保留と通常cloneの構成を検討する

日付: 2026-09-11。状態: 相談・提案。既存機能の削除や単一フォルダ運用への仕様変更は未決定。

ユーザーは[共有hookの具体計画](../../design/shared-worktree-hooks-plan.md)に対して「worktree複雑なら一旦なくてもいいんだけど、どうかな？ ディレクトリ構造はどうなるの？」と相談した。③の計画について、管理元を一つに固定する条件への回答としては扱わず、簡単な構成を比較する。コード・設定・Issue・PRの変更、実機試験は行わない。

## 推奨する当面の進め方

③の新しい共有hook実装はいったん保留し、通常のGit cloneをメインAIの管理用プロジェクトとして使う案を勧める。ステージ、Sensor、会話承認、OKF Knowledge、ADR、Intent stateは維持できる。worktreeの機能やファイルを削除する指示とは解釈しない。

現在のworker割当は、管理元とは異なる実在のGit top-levelを要求する。追加worktreeであることや同じgit-common-dirであることは要求していないため、必要な基準commitを持つ別cloneも使える。reviewerにも管理元とは異なるroot/sessionが必要で、コードを扱う段階では管理元と同じコード版・bytesを検査する。根拠: `src/internal/assignment/reservation.go` の `workerRoot`、`src/internal/flow/unit.go` のclaim/result/integrate、`src/internal/flow/review.go`。

したがって、現行契約を変えずにlinked worktreeを避ける例は次のとおり。通常cloneもGitの広義のworking treeではあるが、ここでは `git worktree add` で作る追加worktreeを使わないという意味である。

```text
projects/
├── my-app/           通常clone。メインAI、hook、進捗・Knowledgeの管理元
├── my-app-worker/    実装用clone。必要な基準commitから作業する
└── my-app-review/    レビュー用clone。対象のコード版を揃えて読む
```

子の起動はメインAIが既存のnative機能で行う。作業場所のcloneを増やすことは、独立した管理元やregistryを増やすことではない。worker側にGit共有文書のcopyがあっても、共有文書と進捗の更新担当はメインAIのままとする。コードの成果はcommitとして回収し、統合後の版を検査する。cloneはGit管理情報も別なので、成果commitの受け渡しが必要であり、通常cloneへ変えれば同期が不要になるとは説明しない。

## メインの管理用cloneの配置例

```text
my-app/
├── .git/
├── .codex/
│   ├── hooks.json
│   └── agents/
├── .agents/skills/
│   ├── aidlc/SKILL.md
│   └── aidlc-cli/SKILL.md
├── src/
└── aidlc/
    ├── workflow/
    │   ├── stage-graph.json
    │   └── stages/
    ├── templates/adr.md
    ├── .runtime/
    └── spaces/<space>/
        ├── intents/<intent_id>/
        │   ├── state.json
        │   └── history/
        └── knowledge/
            ├── codekb/
            │   ├── current-analysis.md
            │   ├── architecture.md
            │   └── <機能名>.md
            ├── design/<intent_id>/
            │   ├── requirements.md
            │   └── implementation-plan.md
            ├── adr/<判断名>.md
            ├── rules/rule.md
            └── log/<intent_id>-work-log.md
```

これは利用プロジェクトの主要な配置例で、現在の開発リポジトリを移動した結果ではない。`.runtime` はローカルの会話・担当管理でGit共有しない。`knowledge` 配下が選択SpaceのOKF検索範囲である。根拠は `src/internal/install/install.go` と `src/docs/user-guide.md`。

## フォルダ一つにする場合との違い

メインAI、worker、reviewerを全て同じプロジェクトフォルダで動かす構成は、現在の別root必須検査に反する。並列実装を止めるだけで、この検査が不要になるわけではない。採用するなら、単独writerの取得・終了、読取り専用reviewの独立性、保存先と進捗の整合を別の具体計画で検討する。ユーザーはまだこの仕様変更や既存検査の削除を承認していない。

今回の共有hook計画は資料として保持し、運用方針の相談が解決するまで実装へ進めない。①②の修正とPR #170の完了には影響しない。
