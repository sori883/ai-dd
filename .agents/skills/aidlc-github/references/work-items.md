# 作業単位を識別し、対応を残す

## 一つのIssueが表す範囲

| 単位 | 一致を確認する項目 | PRが扱う成果 |
| --- | --- | --- |
| Intent | repository、Space、intent_id、単位Intent | そのrepositoryで担当するIntentの受入条件 |
| Bolt | repository、Space、intent_id、単位Bolt、step_id、bolt | その実行回のBoltに属する対象Unit一式 |

現在の公開状態にある`config.units`の`step_id`、`id`、`bolt`を参照する。新しいBolt IDをskillで発行しない。
同じIntentを複数repositoryで実装する場合はrepositoryごとに対応を作り、全体との関係を本文へ書く。
一つのPRへ別repositoryの変更は含められない。親Issueが既にある場合は関連先として記載し、一部のBolt PRで全IntentのIssueをcloseしない。

各Issue／PR本文には、検索の手掛かりとして次のmarkerを置く。IDは実在する32桁のIntent IDに置換する。

```html
<!-- aidlc-intent: 0123456789abcdef0123456789abcdef -->
```

markerだけでは単位を区別できない。本文の項目表と受入条件を必ず照合する。タイトルの似方や先頭の検索結果で決めない。

## Issue本文の例

以下はBolt単位の例。Intent単位ではstep_id、bolt、Unitの行は省略できる。

```markdown
<!-- aidlc-intent: 0123456789abcdef0123456789abcdef -->
## 目的と背景
請求額の計算をAPIから利用できるようにする。現在は画面側に計算があり、他の利用先から再利用できない。

| 項目 | 値 |
| --- | --- |
| repository | example/billing |
| Space | billing |
| intent_id | 0123456789abcdef0123456789abcdef |
| 単位 | Bolt |
| step_id | s04 |
| bolt | invoice-api |
| 対象Unit | calculate、endpoint |

## 対象と受入条件
- 金額計算とAPI入口を対象とする。
- 端数処理と不正入力のテストが通る。
- 既存の計算結果を維持できることを確認する。

## 計画・許可・未確定事項
利用者がAPI化とこの作業分割を承認済み。公開APIの詳細は要件整理で確定する。
実装の許可、工程の成果承認、GitHubへの公開許可を混同しない。

## 検証方法
計算の単体テスト、APIの結合テスト、独立レビューを行う。
```

利用先のIssue templateがあれば構造を合わせる。上の例のrepository、承認やテスト結果を実データとして流用しない。
ラベル・担当者・milestone・reviewerは利用先の指定に従い、ai-dd開発用のラベルを要求しない。
外部へ公開する本文へ、非公開の会話、認証情報、不要なローカル絶対パスを転記しない。必要な背景は本文だけで理解できるよう説明する。

## 任意のOKF対応表

既存の採用済み対応表があればそれを使う。無ければ以下を提案し、GitHub運用の記録として保存する。

- Concept ID: `design/INTENT_ID/github-links`
- 実ファイル: `aidlc/spaces/SPACE/knowledge/design/INTENT_ID/github-links.md`
- type: `GitHubLinks`、intent_id: 対象ID、tags: `github`

これはIssue／PRと作業単位のリンクを保持する文書であり、stateや工程合格の証拠ではない。Sensorの必須成果物へ自動追加しない。
日時やfrontmatterを手書きせず、メインAIが`memory` CLIで保存する。`A`は利用先のaidlc実行ファイル、`DRAFT`はSessionStart等が示す、そのproject内の本文用draft。

```sh
A memory search github --space SPACE --intent-id INTENT_ID
A memory show design/INTENT_ID/github-links --space SPACE
A memory create --help
A memory update --help
```

新規作成例:

```sh
A memory create design/INTENT_ID/github-links --space SPACE --intent-id INTENT_ID --type GitHubLinks --title 'GitHubの対応先' --description 'IntentとBoltに対応するIssueとPR' --tag github --actor process:codex --body-file DRAFT
```

本文には次の対応を必要な行だけ記載する。公開repository名、branch、URLも取得結果を使う。

| repository | 単位 | step_id / bolt / Unit | Issue URL | base / head | PR URL |
| --- | --- | --- | --- | --- | --- |
| example/billing | Intent | 該当なし | 確認済みURL | main / feature/invoice | 未作成 |

更新前はshowで本文・metadata・hashを読み、本文を用意して`memory update`へ`--expect HASH`を渡す。共有保存はメインAIだけが行う。
GitHubの状態は利用時に再取得し、古い対応表だけで完了と判定しない。
GitHub作成が成功して対応表保存が失敗した場合は、取得したURLと未保存を報告し、GitHub項目を再作成しない。
保存先が競合したら再読込みして統合する。memoryが部分保存を報告した場合も、実ファイルの内容と診断を先に確認する。

差戻しでstep_idが変わる場合は、同じ目的を継続する既存Issueへ旧・新実行回の対応を追記できる。
その判断が確定するまでは別作業と決め付けない。承認済みの元仕様、CLI管理の`log/INTENT_ID-work-log`は直接更新しない。
