# ステージ拡張より先に製品4担当を定義する

状態: Accepted。ユーザーが調査・要件整理・worker・reviewerのサブエージェント化を直接依頼した。
調査対象は既存資材と公式ドキュメントの両方。調整役はユーザーとの対話を担い、調査と要件整理を分担する。
専用reviewerだけで調査まで担う構成や、調整役へ調査/要件を集中する前案を、この4担当構成で置換する。

[実装計画](../../design/product-agent-roles-plan.md)で名前、権限、入力/返却、配布、検証、実装許可を具体化した。
調査/要件/レビューはread-only、workerは担当worktreeのworkspace-write。model/effortは利用者設定を継承する。
共有OKF/ADR/stateのwriterは調整役一人の既存契約を維持する。子は結果/本文案を返す。
CLIは新schedulerを作らず、調整役が必要な担当を起動する。4つを毎回起動する強制pipelineにはしない。

[Sensor具体案](../../design/space-reverse-okf-sensor-plan.md)は後続へ保留。
質問済みの鮮度基準・変更不要文書の扱いは未回答のまま残し、今回の4agent実装のblockerにしない。
配布版更新・README全面整理も引き続き残対応。
ユーザーAGENTS未commit差分、未追跡参照資料と既存利用dataを保全する。
