# M1着手前の確認とIntent操作の未決事項

- 日付: 2026-09-08
- 状態: 実装開始の条件付き依頼を受領。Intent公開操作は回答待ち。

ユーザーは「その他不明点がなければ実装に入ってください」と指示した。
既に回答された3点（OKF・資産の内包、新OKF Space生成、rule.mdの作成時コピー）は再確認せず実装条件とする。
旧ロードマップの包括承認を流用せず、現在の最小製品計画を条件付き依頼の対象として確認した。

## 残る利用者向け判断

Intentの開始・再開を本家の名前指定・切替操作に揃えるか、現案の固定IDとsessionへの明示bindへ変更するかが未確定。
先行設計ではランダム128 bit ID、`kdr create`、`session bind`を提案したが、3点のA回答はこの公開操作を対象としていない。
本家固定2.6.123のIntentはUUIDv7と名前由来の識別を持ち、作成・切替時に選択を更新する。
新方式のKDR配置・旧state非依存は合意済みだが、これだけで利用者向けの操作変更まで承認されたとは扱わない。

質問は、現案の固定ID発行と会話への明示bindによる開始・再開へ変更してよいか。
本家操作を維持する回答の場合も、旧state・registryの移行は要求せず、合意済みKDR保存に合う計画へ改訂する。
この確認が終わるまでコード・設定・Issue・PRの変更は開始しない。

## 技術確認

read-onlyのproject_plannerとtechnical_researcherで着手可否を確認した。
mainは `fc8bd7ea633f0c1ad8cc605233572a6106b3b911`、Open PRなし。既存の未コミット文書を保持した。

固定 `codex-cli 0.153.4` のローカル実行ファイルに必要hook schemaが含まれ、hooks featureはstable/enabled。
実行ファイルSHA-256: `b973d440acac501fd2594a43e7ca9ce41e0a65b9dfb28d0d7a7837c99e1261e3`。
SessionStartにはturn_idがなく、sessionの読込済み印を無効化するために使う。
turnはUserPromptSubmitから得る。Pre/Postにはsession_id・turn_id・tool_use_idがあり、Stopにはstop_hook_activeがある。
これは実行ファイルの形式確認であり、liveでの発火・拒否・非同期終端・Stop再入の実証ではない。
実装開始時にfresh sandboxで実証し、不成立なら契約を弱めず対応環境を確認する。

独立した計画確認で挙がった補強は、内包資産の配置とfresh導入の衝突検査、Space bundle単位lock、
本文保存後のindex/log部分失敗の修復手順である。これらを実装handoff前に自己完結した計画へ具体化する。
型名・package名などの通常の実装詳細を追加のユーザー質問にはしない。

根拠: [現在の設計案](../../design/minimal-product-contract-m0.md)、
[3点の回答](2026-09-08-single-binary-space-rule-accepted.md)、
[本家Intent作成契約](../research/2026-09-01-intent-create-contracts.md)、
[Codex hooks公式補助資料](https://learn.chatgpt.com/ja-JP/docs/hooks)。
