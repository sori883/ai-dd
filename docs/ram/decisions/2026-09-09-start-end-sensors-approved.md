# ファイル別の開始・終了Sensorを実装する

状態: Accepted。ユーザーは各段階の開始/終了で検査するファイル表を確認し「はい、実装してほしいです」と直接依頼した。
[実装計画](../../design/start-end-sensor-plan.md)にCLI、文書型/節、state、TDD、検証と実装許可を具体化した。

開始は入力準備と前段合格版、終了は成果と現在対象の独立reviewを確認する。
現状解析と構成図はSpace共有、要件と計画はIntent別。必要なADRと統合後の現行仕様も検査する。
共有文書の日時だけを更新せず、参照版と現在対象を照合する。開始成功後の実装や共有文書更新を旧入力hashで一律拒否しない。
開始確認を保存するbegin、読取り専用のcheck --boundary start|end、現段階entryと最大4件の直近acceptedを追加する。
全操作audit、本文snapshot、receipt台帳、別工程体系は追加しない。

[共有現状解析の合意](2026-09-09-space-shared-analysis-and-diagram.md)と今回表に基づき、
旧案の『工程開始時刻以降のgenerated.at』を追加の必須gateとする検討を置換する。
共有文書は参照版の確認と独立reviewで扱い、Intentごとに複製/ID上書き/空更新しない。
新規schema2を対象とし、旧Intentは移行不要の合意どおりファイル保持と明示エラーにする。

実装はGo単一バイナリ、既存依存のみ。単独writer、1 Issue/PRで実施し、独立reviewとfinal/CI後に自律マージする。
固定AI-DLC2.6.123の33Stage/receiptからの最小4段階への変更を維持し、今回の開始/終了の役割を反映する。
配布更新・README全面整理は別の残対応を保持。ユーザーのAGENTS未commit差分と未追跡参考資料は変更しない。
