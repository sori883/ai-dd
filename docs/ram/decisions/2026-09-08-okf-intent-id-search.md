# OKF検索でintent_idを指定できるようにする

- 日付: 2026-09-08
- 状態: Accepted（ユーザーの直接指示。公開文法と検証条件を具体化）

ユーザーは標準OKF metadataの利用について「はい、お願いします」と回答し、
さらに「intent_idでもOKFを検索出来るようにしておいてほしい」と指示した。
[標準metadataの説明補足](2026-09-08-kdr-okf-metadata-clarification.md)を採用方針とし、
以下をM1の検索契約へ加える。

## 利用者が得る結果

名前を変更した後でも、固定IDから対応するKDRを検索できる。
専用の検索条件を設け、本文やタグにたまたま書かれたIDとの混同を防ぐ。

```sh
aidlc memory search --space main --intent-id 9c2f0b1d7a684e55a19d0680f731db26
```

これは実装予定のコマンドで、現在実行可能という意味ではない。

## 検索契約

- 指定SpaceのOKF bundle内で、frontmatterの `intent_id` が指定値と完全一致する文書を返す。
- IDだけで検索できる。通常の検索語も指定した場合は、ID条件と検索語条件の両方を満たす文書を返す。
- IDの省略時は従来提案の通常検索を行う。明示的に空のIDを指定した場合や、32桁の小文字16進数でない値は入力エラーにする。
- 該当なしは正常な0件。別Spaceや本文・タグの文字列一致で代用しない。
- 結果にはConcept ID、intent_id、title、description、bundle相対pathを含め、本文表示へ進めるようにする。
- 検索は読取り専用で、会話のIntent選択やhookの未記録表示を変更しない。
- KDRを開始・再開する際は、検索結果からtype・IDとファイル名の一致も検査する。同じIDのKDRが複数ある等の異常を勝手に一件選んで隠さない。
- OKFに元から存在する標準検索演算子とは主張しない。合意済み拡張項目intent_idをaidlc内の検索処理が扱う製品機能である。

ID検索のために全操作audit、別のIntent registry、永続検索cacheは追加しない。
ルール・共有設計にintent_idを一律付与する要求でもない。

## M1での検証

対応KDRの取得、title変更後も同じIDで取得、非一致・該当なし、別Space隔離、
本文だけにIDがある文書の除外、通常検索との併用、入力形式違反、読取りでsessionが変わらないことを検証する。
frontmatterの読込・更新往復でintent_idと標準metadataが失われないことも確認する。
対象は既存M1案の `src/internal/minimal/` 検索処理、`src/internal/cli/` 公開文法、
KDR保存・一周検証と関連文書。新しい外部Go moduleや外部サービスの導入は含めない。
