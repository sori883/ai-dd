# 現行知識のフォルダをcodekbへ変更する

状態: Accepted。ユーザーの「knowledge/knowledge これをknowledge/codekbに変更して」という直接実装依頼。

利用プロジェクトの現行仕様・解析・構成図の配置を`aidlc/spaces/<space>/knowledge/codekb/`へ変更する。
外側のknowledgeはOKF Bundleの検索範囲として維持する。design/adr/rules/logとstateの配置は維持する。
CurrentAnalysis/Architecture/Knowledgeなどのtypeやintent_idの検索契約は変更しない。
新規配布、Space作成、Sensor/修復hook、CLI例、現在手順、関連fixtureを同じpathへ揃える。
既存利用先のファイルを削除・自動移動しない。旧Intent/旧配置の移行と二重運用は不要という既存判断を維持する。

これはユーザーが指定した保存先変更とそれに必要な参照修正の直接承認であり、旧33 Stage承認を流用しない。
過去のRAM・調査snapshotは履歴として保持する。詳細は[実装計画](../../design/knowledge-codekb-plan.md)。
