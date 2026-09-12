# 本家の配布構成を使う3環境製品対応の実装依頼

2026-09-12。Accepted（ユーザーによる直接の実装依頼）。

ユーザーはAI-DLC製品自体をCodex、Claude Code、VS Code GitHub Copilotへ対応させる方針に対して「はい。ai-dlc本家に配布の仕組みがあると思うのでそれを流用しつつ、実装してください」と依頼した。Copilot CLIではなくVS Code版という前回答も維持する。[前回調査](../research/2026-09-12-three-agent-environments-feasibility.md)の実装未承認という状態を、この依頼が置換する。

[具体計画](../../design/three-host-distribution-plan.md)に、固定本家2.6.123の共通coreと環境別manifest/生成という配布構成、Go単一バイナリ、既存工程・OKF・同root worker予約を維持する範囲を記録した。まず既存Codexの配置bytesを保持して配布処理を共通化し、独立した専用環境で新hostのイベント識別を確認する。D1は保存形式と利用者動作を変えず実装できる範囲である。

子ID・会話回答ID・hook共存・環境切替などの重要契約に根拠不足があれば、影響する後続実装を止めて具体的に確認する。旧33 Stageの包括承認や過去の保存形式に対する互換不要の回答を流用しない。任意GitHub skillは標準配布へ追加せず、日本語lintは別CLI、外部Go module追加なし。元checkoutの未commit資料は変更しない。
