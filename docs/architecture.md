# アーキテクチャ

AI-DLCは、一つの目的をIntentとして保持し、理解（discovery）、実装計画（planning）、
TDD（tdd）、統合検証（integration）の順に進める単一Go実行ファイルです。
各境界と完了は、現在の成果物を検査するSensorと独立reviewの両方で確認します。

`src/internal/flow` が現在のstate、比較保存、Sensor、review受理、段階遷移、Unitを管理します。
正本は `aidlc/spaces/<space>/intents/<id>/state.json`。Git共有するため、調整役AI一人が更新します。
`revision` の期待値が一致しなければ保存せず、同じディレクトリの一時fileから置換します。
未知field、重複JSON key、不正identity、破損dataは復元を推測せず診断します。

Sensorの対象hashには、段階、計画、成果物本文、実コード版、Unit成果commitを含めます。
stateの保存revisionやreview結果の更新だけで対象hashは変わりません。実ファイルが変われば
古いreview passは使えません。未知・循環するUnit依存、存在しない統合commitも拒否します。

調整役AIが別worktreeのworkerと別rootのread-only reviewerを起動します。製品Goは起動しません。
Unitは担当範囲、検証、依存とBolt（作業のまとまり）を持ちます。依存の統合前や同一worktree・
範囲の重複割当は開始できません。結果は現在のrun/session/rootと照合し、実commitの祖先関係を確認します。
中断したrunは `needs_confirmation` となり、実環境を確認してから再開します。
reviewの独立性は運用上のものです。同じOS権限の相手に対する完全な著者認証ではありません。

`src/internal/minimal` は公開操作とCodex hookを接続します。session選択、Ruleの全文読込hash、
実行中tool IDだけを一時状態に保持します。一般toolには選択と現在turnのRule読込を要求し、
同じIDのPostで実行slotを解放します。失敗した編集がPostを返さなければ、AIが終了を確認し、
同じsession/Space/Intentへの明示 `session bind --recover` を使います。Stopは実行中toolを診断します。
毎操作のKDR保存義務、全操作audit、製品によるtranscript解析はありません。

`aidlc/.runtime/` は内部 `.gitignore` で無視するローカル状態です。shared stateと混同しません。
`src/internal/filestore` はroot境界、symlink拒否、256 KiB上限、単一file置換、ローカルlockを提供します。
`workspace` はSpace/root選択と非上書き生成を担当します。

Knowledgeは現行のwhat/how、`knowledge/ADR/` は判断のwhyです。一般Knowledgeのartifact区分と
OKF `type` は別物で、Designなど内容に応じたtypeを保持できます。必要なADRだけ作成し、
不要なら理由をreviewします。`okf` と `okfmemory` は固定OKF v0.2のmetadataを検査・保持・検索します。
外部依存は承認済み `go.yaml.in/yaml/v3 v3.0.5` のみです。

配置原稿は `src/core/minimal` と `src/harness/codex/minimal`。installがfresh projectへ配置し、
実行時は配置済みRule/skillを読みます。埋込み原稿へのfallbackはありません。
入口skillは4 KiB以内、必須Rule本文は16 KiB以内とし、超過時に切り捨てません。

公開CLI/schemaは [詳細契約](design/four-stage-workflow-contract.md)、除去した旧製品経路は
[除去記録](design/four-stage-removed-product.md) を参照してください。旧利用dataを移行・削除しません。
