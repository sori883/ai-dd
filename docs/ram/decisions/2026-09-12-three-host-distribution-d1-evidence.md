# 3環境向け配布基盤D1の実装証拠

2026-09-12。Issue #183、実装許可は[直接依頼](2026-09-12-three-host-product-implementation-approved.md)。[具体work unit](../../design/three-host-distribution-d1-work-unit.md)のD1を単独Go担当が実装した。独立review・final・CIの最終結果は対応PRで確認する。

共通のManifest.Renderが原稿FSと配置対応を展開し、相対path・重複・file/親directory競合を検査する。Codex.Distributionが既存配置表を組み立て、環境固有のhook生成をemit.goへ分離した。installerは完成資材を全件事前検査してから新規保存する。既存原稿、工程、保存形式、CLI、relocate、日本語補助CLIは変更しない。

旧main f8d9eb0d83143144bbcb2fc5b6a9dc5db80acf94のinstallerから、69fileのpath・SHA256・modeを採取した。新rendererで期待値を再作成せず、変更後の配布結果と全件比較して一致した。bootstrapのbinary引用・hook設定も含む。

d1-01の旧動作固定はALREADY_GREEN。最初にWalkDir順と全path sort順を同一視したtest誤りがあり、親が採取結果のPath整列へ訂正した。製品のREDとして数えていない。d1-02は空scaffoldの正常展開・不正入力がassertion REDになり、renderer実装後GREEN。d1-03はDistributionの空出力がREDになり、分離後GREEN。d1-04の既存読込み・移転・標準skill検査も成功した。

親はwork unit末尾に全差分を確認し、`go test -count=1 ./src/harness/... ./src/internal/install` と対象app読取りtestを再実行して成功した。外部module追加なし。Go担当はreview前にgofmtを適用した。

D1は3環境の製品対応全体を完了させる変更ではない。[接続前提確認](../research/2026-09-12-three-host-hook-preflight.md)ではClaudeの正常・拒否・失敗・追加依頼を実測し、VS CodeのUI実機確認と重要な接続選択を残している。

## 独立reviewでの訂正

最初のreviewは製品コードにP0/P1なし。比較testにアポストロフィ付き一時rootの正規化漏れと、umask 077のmode偽陽性というP2があり、同じ単独担当がtestのみ修正した。rootはshell引用とJSON escape後のtokenだけを正規化し、modeは同じ作成条件のroot外controlfileと比較する。元69fileの期待SHAと製品codeは変更していない。親も通常時とumask 077の対象testを確認した。

D2には追加の確認gateがある。Claudeの子完了通知がUserPromptSubmitとして届くことを実測したため、イベント名だけで人間承認を認定しない接続契約が必要。G0の詳細記録を参照する。
