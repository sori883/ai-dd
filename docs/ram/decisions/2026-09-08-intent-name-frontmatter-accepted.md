# Intentを名前で選び、KDRのfrontmatterに識別情報を保存する

- 日付: 2026-09-08
- 状態: Accepted（Intent操作とfrontmatter配置の合意）

## 背景と回答

先行案は固定IDと会話への明示bindを利用者の操作として提示していた。
ユーザーから背景の説明を求められ、内部の識別と利用者の操作を分けて説明した。
「名前でIntentを作成・選択し、内部で同じKDRを特定する方式」を提案し、
ユーザーは「はい、良いです。frontmatterに設定すると利用しやすいと思います」と回答した。

## 決定と具体化

利用者は作業名でIntentを作成・選択する。普段は固定IDやbindを入力する必要をなくす。
AIがCLIを使って選択を解決し、内部で現在の会話とSpace・Intent IDを結び付ける。
KDR冒頭のYAML frontmatter（文書の識別情報を書く部分）には、次を保存する。

```yaml
---
type: KDR
intent_id: 9c2f0b1d7a684e55a19d0680f731db26
title: 検索結果をタグで絞る
description: タグ絞込みの目的・判断・検証と残件を記録する。
tags: [kdr, search]
status: draft
---
```

- `intent_id` は作成時にCLIが発行する不変の識別子。ファイル名のIDと一致させる。
- `title` は既存のOKF標準項目をIntent名として使う。別の名前項目を重複して作らない。
- 保存先は `aidlc/spaces/<space>/knowledge/kdr/<intent_id>.md`。
- タイトルの変更、試作、テスト、修正、別の会話からの再開でIDと保存先を変えない。
- 名前での選択は指定Space内で行う。同名が複数ある場合は勝手に選ばず、候補とIDを表示する。
- 会話IDはKDRに固定しない。一つのIntentを別の会話から再開できるよう、会話との対応は既存案の一時管理で扱う。
- IDとファイル名の不一致は通常の更新で受け入れず、同じIDへの修復として扱う。改名だけで別Intentを作らない。

上記の項目名や重複時の安全な解決は、ユーザー指定を実現する具体化である。
`intent_id` は製品が使うOKF拡張項目であり、OKFの標準必須項目やConcept IDではない。
OKF Concept IDは引き続きbundle相対pathの `kdr/<intent_id>` である。
固定OKF v0.2仕様のExtensionsは追加キーを許す。未知のmetadataを更新時に保持する契約も維持する。
`status` は文書の成熟度であり、工程stateを意味しない。

## 置換する判断と実装許可

[Intent操作の未決事項](2026-09-08-m1-readiness-intent-operation-question.md)は、この回答で解消した。
同記録は質問の履歴として保持する。
[先行契約案](../../design/minimal-product-contract-m0.md)の「IDはファイル名のみ」「独自intent_idを使わない」
および利用者に固定IDとbindを要求する操作を置換する。
[初期OKF統合境界](2026-09-03-okf-reference-boundaries.md)の製品固有metadataを追加しない方針は、
今回ユーザーが指定したKDRのIntent識別情報に限って変更する。工程や全操作auditのmetadataを増やす許可ではない。

本家AI-DLC固定2.6.123は名前による選択とregistry・cursorを使う。
採用方式も利用者は名前で選ぶが、保存上の正本はOKF KDRとし、会話単位で選択する。
目的は名前で扱いやすくし、旧registryや工程stateなしで同じ作業へ戻れるようにすること。
旧方式との互換性・移行は要求せず、既存ファイルは削除しない。

「その他不明点がなければ実装に入ってください」という先行の条件付き実装依頼は維持する。
本回答をM2/M3や旧33 Stageの実装許可へ広げない。M1開始前には、名前からの選択・frontmatterを反映した
自己完結した計画と保存・配布・hookの検証範囲を整合させる。

根拠: ユーザーの本タスク内回答、[固定OKF v0.2仕様 §4.1 Extensions](../../okf-analysis/upstream/SPEC-v0.2.md)、
[本家Intent作成契約](../research/2026-09-01-intent-create-contracts.md)。
