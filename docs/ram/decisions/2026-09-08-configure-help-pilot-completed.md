# configure helpの実案件を四段階で完走した

状態: 製品Intentはcompleted。PRの最終検証とGitHub checksはこの記録作成後のgate。
Issue #135。ユーザー指定の実案件を、本会話のAIと独立subagentが実CLIで実施した。
新しいCodex CLI会話の自動hook発火を観測したliveではない。

## 結果

Intent IDは `6956086f4c4fa09074715f2949b8c28e`。目的はconfigure helpへ有効な設定JSON例を追加すること。
隔離した本リポジトリのworktree `/private/tmp/ai-dd-configure-help-pilot` に製品を配置し、
本会話のsessionへbindして必須Rule全文と配置WORKFLOWを読み、同じIntentで進めた。
stateは製品CLIだけで更新し、レビューのpassを先回りして書かなかった。

| 境界 | 実施と証拠 |
| --- | --- |
| discovery | 現状・目的・範囲・受入・ADR不要理由を設定。Knowledgeを本文とmetadata引数から作成。Sensorと別rootの独立レビューpass後、planningへadvance |
| planning | 単独直接実装、順序付きTDD、2つのtargeted commandを設定。Sensorと独立レビューpass後、tddへadvance |
| tdd | JSON例0件でRED→2例と型説明の追加でGREEN。両例の実CLI configureとplanning Sensorも成功。実装commitを設定し、現行Knowledgeへ更新。Sensorと独立コードレビューpass後、integrationへadvance |
| integration | 配置Skillと同じbinary pathを最新codeでbuildしnative helpを取得。2例のJSONとUnit数、Knowledge検索、実CLI testログを確認。Sensorと独立レビューpass後、revision18/status completed |

アーキテクチャ変更がないためADRは新設せず、不要理由をstateへ記録した。
Knowledgeは `aidlc/spaces/default/knowledge/knowledge/configure-help.md`、検証成果物は `aidlc/evidence/`。
製品の利用データ・絶対path入り配置設定は実案件worktreeに保持し、製品PRへ混ぜない。
コード・Knowledgeを検査したHEADは `5083c88a198d51b270e212d3ed07875a31a9fe61`。
この完了記録を追加する後続commitと、製品の受入を確認したコード版は区別する。

## 実利用で確認した点

- 最初のreview checkoutには変更済みRAM索引のコピーが不足し、CLIがbytes不一致を拒否した。
  同じHEADと対象bytesへ揃えて再割当し、拒否を迂回しなかった。
- 実装後のKnowledgeに旧説明が残ったため、CLIで現在の使い方へ更新した。
  変更後のtargetへ再割当し、独立reviewerが解消を確認した。古いpassを再利用しなかった。
- 日常運用PR #136の取り込みでRAM索引が競合した。両記録を保持して解消し、取り込み後の版をレビュー対象にした。
- 小さな改善はUnit分割なしで進められた。Unitありの利用例はintegration testで確認した。

## 境界識別子

- discovery target: `3112adb4e589ff835071685eb9874b7ea80e99291348c9c495f140e5b6897ced`
- planning target: `a489f274978c351cea65cadd5ad360baa88262c184fa6fc1c2432a0cfee2b071`
- tddのKnowledge更新後target: `ee0bc0469f6d624d1b98728d7f7847d75903dd6c3d83750d7842bac11f32bbe2`
- integration target: `6767b506ba12ee254600a7ebb407bdb7ddd2f7a397c9eb531f2a260fff92fe5c`

独立review sessionはpilot-reviewer。discovery/planningは別review worktree、tdd/integrationは
`/private/tmp/ai-dd-configure-help-review-tdd` でread-only確認。親が実報告をCLIへ受理した。
受理後stateのJSONは実行ホストの `/tmp/pilot-discovery-reviewed.json`、`/tmp/pilot-planning-reviewed.json`、
`/tmp/pilot-tdd-reviewed.json`、`/tmp/pilot-integration-reviewed.json`、`/tmp/pilot-completed.json` にも保持する。
