# PR作成・更新

PRを、レビューと将来の実装状況確認に使える日本語の変更記録として作成・更新する。

## 作成前

1. repository、default/base branch、head branch、remote、認証状態を確認する。
2. head branchに対応するOpen・Closed・Merged PRを検索する。同じheadまたは同じIssueのPRが
   既にあれば、原則として新規作成せず既存PRを確認・更新する。
3. 紐づくIssueがあり、承認済み範囲と受け入れ条件を保持していることを確認する。
4. base...headのcommit、diff、変更ファイル、worktree、push状態を確認する。
   無関係な変更や未pushの必要commitがあれば、PR作成前に解消する。
5. 必須テスト、独立レビュー、既知の失敗を確認する。未実施を成功と書かない。

## タイトルと本文

タイトルは日本語を基本とする。`feat:` や `docs:` などのprefixを使う場合も、以後を
日本語にする。

リポジトリの `.github/pull_request_template.md` があれば構造を再利用し、少なくとも次を
現在の差分に合わせて記載する。

- 概要と利用者向けの結果
- `Closes #<番号>` など、マージ時にcloseするIssueとの明示的な紐づけ
- 実装した機能と主要な変更箇所
- 実行した検証と結果。cross-buildとnative実行など、保証の違いを区別する
- 独立レビューの結果と、修正したblocking finding
- 安全性、互換性、移行、利用条件に影響する特殊な制約がある場合だけ、その特記事項
- 本家AI-DLCから意図的に変更した仕様・挙動がある場合だけ、比較version、理由、影響
- 自動マージを設定していないこと

PR本文は実際の差分、Issue、検証証拠を要約する。Issue本文やcommit messageの自動転記だけで
済ませず、実装した内容を本文だけで確認できるようにする。通常の未実装機能や「今回は
作らないもの」は列挙しない。確認していない互換性、性能、coverage、OS上の動作を断定しない。

## 作成・更新後

1. userまたは承認済みフローが許可したhead branchだけをpushする。
2. baseとheadを明示してPRを作成し、自動マージを設定しない。
3. 作成後にPRを読み直し、番号、状態、base/head、Issue紐づけ、本文、checks、URLを確認する。
4. 実装、検証、レビュー、特殊な制約が変わった場合は、既存の人間による追記を保ちながら本文を
   最新事実へ更新する。
5. ユーザーへPR番号、URL、状態、checksの確認範囲を日本語で報告し、merge判断を委ねる。

merge後はPRの `mergedAt` とmerge commitを確認する。Issueが自動closeされていない場合、
作業が完了していることを確認してからcloseする。
