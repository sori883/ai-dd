# Claudeアダプターの既存fixture修復と接続境界

2026-09-12。Issue #185、work_unit_id `claude-connection-d2`、verification_mode `loop`。製品対応、新規配置、環境別アダプターという直接承認の範囲で継続する。追加機能の承認を求める変更ではない。

C1〜C7の対象testはGREEN、C8の公開CLI journeyもexit 0だった。C8はALREADY_GREENとして扱う。途中のconfigure JSONの誤りや、末尾に判明した旧入口・旧schemaのfixture失敗を製品REDには数えない。末尾の変更package確認は失敗しており、実装完了・独立review・final成功とは報告しない。

[具体計画](../../design/claude-code-connection-plan.md)に、bootstrap、配布集合、hook reliability probe、直接helperのイベント指定、assignment schemaのfixture修復を追記した。唯一の実装担当が同じwork unitで修復する。製品の許可/拒否の観測目的は維持し、保存形式の変更だけに旧期待を合わせる。

同じ計画には、既存Codexの役割・親宛先検査を維持し、Claude固有のStart結合をCodexへ要求しないことを明記した。未回答質問の失効は現在選択したsession/Intentと保存済みrequest/tool IDを照合し、置換済みの古い質問も片付けられるようにする。承認登録前の回答証拠は新質問で上書きせず保持する。これらは既存の承認・復旧要件を安全に実現する具体化である。

未完了はfixture修復、質問保存の競合と未登録証拠の保持、未知toolの拒否、環境固有pathの分離、導入文書、末尾対象確認。その後に製品アダプターを使った固定実機G0、独立review、read-only final、PR checksを実施する。
