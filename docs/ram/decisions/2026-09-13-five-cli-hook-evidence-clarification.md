# 五CLI分離で維持するRule照合とPostの責務

2026-09-13。Issue #200の実装時に、計画の「Post処理によるRule内容照合・読取り証拠」という現行説明の誤りを確認した。
開始commit `64833d7e94203be0284c18abbf4a35b23810cf77`の`src/internal/app/flow.go:213-240`はbindFlowでRule本文・hashを返しRuleHash／RuleTurnを保存する。
`hook.go:83,92-105`は読取り例外と次PreでのRule照合、`hook.go:129-134`は一致tool IDでslotを解放する。
親が同じ根拠を確認し、承認済みの既存保護維持の範囲で計画説明を訂正した。
新たな`okf rules`のPost出力認証や永続証拠は追加しない。実機試験observerのPre/Post観測とは区別する。
[承認](2026-09-13-five-cli-release-011-approved.md)と[計画](../../design/five-cli-release-011-plan.md)の分離方針は維持する。

実装work unit `five-cli-release-011`のslice1は旧入口拒否・新OKF引数／help／版表示でRED→GREEN。
slice2は新OKFサービスの本文・日時・CAS競合・保存失敗でRED→GREEN。既存metadataとIntent検索の契約はALREADY_GREEN。
各計画targeted commandは終了1の意図した失敗から終了0へ到達し、slice2後にslice1を再実行して終了0。
現行説明の不一致で停止した後、親の上記判断によりslice3から再開した。
