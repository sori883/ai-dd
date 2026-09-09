# ADRのフォルダ名とtypeを小文字にする

ユーザーは配置先 `knowledge/ADR/` を `knowledge/adr/` に変更するよう依頼し、続けてfrontmatterの `type` も小文字にするよう明示した。採用する表記は `type: adr`。

これまでの大文字フォルダ `ADR/` と `type: ADR` の指定を、この2点について置き換える。設計判断の理由を記録するという文書の役割は維持する。他の文書typeの小文字化は指定されていない。

[入出力の実ファイル名指定](2026-09-09-explicit-stage-document-paths-request.md)の例も `knowledge/adr/order-storage.md`、`type: adr` とする。Intentごとの可変ファイル一覧の指定方式は引き続き未確定。

本タスクでは合意を記録する。CLI・Sensor・テンプレート・配布手順などの実装反映、既存ファイルの改名はまだ行っていない。人間承認機能の中断も維持する。
