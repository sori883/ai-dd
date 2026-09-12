---
name: aidlc-github
description: AI-DLCのIntentまたはBoltをGitHub Issueに対応付け、作業の成果が揃ったらPRを提出する任意skill。利用プロジェクトでの開始、再開、Issue更新、PR提出と状況確認に使う。AI-DLC作業単位に関係しない一般的なGitHub操作には使わない。
---

# Intent・Boltの作業をGitHubへつなぐ

このskillを採用したプロジェクトで、作業開始時にIssue、成果が揃った時点でPRを作る。
Intentは一つの目的、Unitは分割した作業、BoltはUnitのまとまり。進捗の正本はAI-DLCのstateであり、GitHubは変更を共有・レビューする場所である。

## 利用条件と担当

- 本体に追加コマンド・hook・工程・stateを登録しない。GitHubを使わないIntentにも導入を要求しない。
- Issue作成・公開、commit/push、PR作成は、利用者の依頼または採用済みのプロジェクト運用に含まれる範囲で行う。skillの配置だけを公開許可にしない。既存の許可を毎回取り直さない。
- メインAIがGitHub操作と共有Knowledge保存を担当する。子は本文案・実装結果・検証の証拠を返す。対象Git作業場所を編集中の担当がいれば、変更を確定させてからcommitする。
- Git・認証済みのghが必要。不足時は草稿と不足条件を返し、勝手にインストールや認証・権限追加をしない。
- AI-DLCの実行ファイルは、利用先のaidlc skillに示されたものを使う。以下の`A`はその実行ファイルを指す略記で、文字どおりのコマンドではない。

## 最初に対象をそろえる

1. 選択中のSpace、Intent ID、現在の手順と承認状態を確認する。`A intent show --help`と`A intent procedure --help`を必要時に読む。
2. GitHub repositoryと、その変更をcommitするGit作業場所を特定する。AI-DLCの管理rootとアプリのGit rootは同じとは限らない。複数候補を推測で選ばない。
3. Issueの単位を明示する。基本はIntent単位。利用者指定や承認済み計画がBolt単位なら、その実行回のBoltを使う。一律の親Intent Issue・全Bolt Issueの二重作成や、Unitごとの自動分割はしない。
4. 対象repository、公開してよい範囲と受入条件を確認する。base/head branchがIssue作成時に未確定なら未確定事項として残し、commit/push・PR作成前に確定する。複数repositoryの成果は各repositoryでIssue／PRを作り、相互に対応付ける。

同じIntentでも、Boltには実行回のstep_idとbolt値が必要。名前だけでは対応を決めない。
具体的な識別と保存は[作業単位と対応表](references/work-items.md)を読む。

## 作業開始・再開

- 開始時: 目的と対象単位が決まり、Issue作成が許可されたら既存項目を探す。見つからなければ目的・受入条件・未確定事項をIssueへ記載する。Issueがあっても実装や工程の承認を得たことにはならない。
- 再開時: 対応表とGitHubの実際の状態を読み直す。同じ目的の未完了Issue／PRを優先し、番号を失ったことだけで新規作成しない。
- 計画変更時: 許可された範囲を本文へ反映する。再実行でstep_idが変わったら、前のBolt Issueを自動複製・close・reopenせず、同じ作業の継続かを確認して対応を残す。
- GitHub操作の引数、検索、失敗からの復旧は[GitHub操作](references/github.md)を読む。

## 成果がそろったらPRを出す

対象の実装、受入条件、必要な検証・独立レビュー・成果承認を確認する。Boltの一部Unitだけを全体完了として提出しない。
PRを出せるかの判断と本文は[PR提出と取り込み](references/pull-request.md)を読む。

現行hookはactiveかつbegin済みの工程で通常操作を許可する。PR提出は成果確認後、Intent単位なら最後の工程、Bolt単位なら対象TDD工程の`intent finish`より前に行う。
finishは次工程へ進む際に現在のUnit一覧を消去するため、Boltの対応と成果はその前に確認・保存する。
「IntentをcompletedにしてからPRを作る」という順序にはしない。承認待ち・paused・completedで止まる場合は、通常の承認・再開手順に戻る。
既に終了したIntentのGitHub作業では再開が必要なことを説明し、利用者の意図を確認する。hook対象外のtoolへ乗り換えて回避しない。

## 保存と報告

GitHub作成後はURLを読み直して確認し、任意のOKF対応表へ保存する。前の承認済み要件・計画、CLI管理のwork-log、stateやindexを直接編集しない。
対応表の変更も検証範囲に入る場合は、変更後のhash・検証・必要な承認をそろえる。

Issue／PRの番号・URL・現在状態、検証と未完了事項を短く報告する。PR作成はmergeではなく、Issue closeはIntentやBoltの機械的な完了判定ではない。
PR作成の依頼からmerge、branch削除、Issue closeを推測しない。追加の操作が既に承認済み運用に含まれる場合だけ、その運用とGitHubの保護規則に従う。
