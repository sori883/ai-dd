# KnowledgeのfrontmatterをCLIで生成する

状態: 要求はAccepted。公開CLIの詳細と更新方法は下記の提案であり、実装前に確認する。

## ユーザーの要求

Knowledge配下にAIがファイルを作る際はCLIで作成し、frontmatterを設定する。
日時は自動、その他の値はCLI引数、本文はAIが書く。AIごとにfrontmatterの書式がばらつくことを防ぐ。
Knowledge・ADR・RuleをOKFで扱う既存方針と、intent_id検索を維持する。

## 現状

PR #131までのmemory create/updateは、AIがfrontmatter付き草稿を渡す方式。
generated.byは--actor、generated.atはCLIの現在UTC時刻で上書きするが、他の項目は草稿依存である。
この文書は作成・更新の入力責任について、その従来手順の変更を求める記録である。
過去の実装証拠やOKF metadataの採用合意を取り消さない。

## 提案する契約

- AIは本文のみの草稿を書く。CLIが引数からmetadataを組み立て、本文と合わせて検査・保存する。
- 作成時の基本引数はtype/title/description/actor。tagsは繰返し--tag、statusとintent-idは明示引数。
  status省略は既存OKFのstable、intent-id省略は未設定とし、共有Ruleへ一律にIntentを付けない。
- generated.atは保存時の現在UTC時刻、generated.byは--actorとする。意味はOKF v0.2の最終内容生成時刻。
  作成日時や検証済み日時を別の意味として捏造しない。
- 本文は--body-fileで渡す案。正本の空ファイルを先に作らず、一回のCLI操作で完全なOKF文書を保存する。
- 更新も本文だけを渡し、未指定metadataと未知metadataを保持する案。変更するmetadataだけ引数で指定し、
  --expectによる競合拒否を継続する。YAMLはCLIが生成する。
- sources/resource/verified/stale_after等の追加項目は既存OKF対応を損なわない入力方法を計画で確定する。
  generated.atと、出典の日時・期限・実検証日時は意味が異なるため、すべてを現在時刻へ置換しない。
- 本文入力がfrontmatterを持つ場合は拒否し、二重の入力元を作らない。内容の正しさはAI・レビューが判断する。
- index/log等の管理文書は既存のCLI管理経路を維持する。OS権限による全書込み禁止を追加する意味ではない。

## 次に具体化する実装範囲

公開文法とhelp（src/internal/cli）、作成・更新処理（src/internal/minimal/command.go）、
metadata生成・検査（src/internal/okfmemory）、配布手順（src/harness/codex/minimal）、
対応テストとdocs/development.mdを対象候補とする。Go単一バイナリ、外部Go module追加なし。

引数からの作成、本文のみの更新、文字列のYAML安全変換、日時自動設定、metadata保持、
不正入力・競合・保存失敗、Knowledge/ADR/Ruleの実CLI一周を受入条件にする。
完全な引数契約、旧--file経路の扱い、検証コマンドと許可範囲を実装計画で提示してからコードを変更する。
本記録だけで新しいCLI全体の実装承認を得たとは扱わない。
