# 初回の操作案内をSessionStartから渡す

- 日付: 2026-09-08
- 状態: Accepted implementation detail（M1直接承認内の初期案内接続）
- 対応Issue: #128

未選択の会話では一般Bash操作を拒否し、固定aidlcの既知CLIだけを例外にする。
その状態でAIがcat等を使って最初のskill本文を読もうとすると、作業開始に必要な手順自体を読めない。

これを解消するため、SessionStartで配置済みの最小skillから短い開始・再開手順を読み、
additionalContextとしてAIへ渡す。skillの原稿を別の場所へ重複して持たず、配置済み本文を使う。
この案内は4 KiBを上限とし、hook出力のadditionalContextLimitを十分に設定する。
欠落・超過は診断し、内包版へのfallbackや黙った切り捨てはしない。

必須Ruleの本文は従来どおりaidlcの明示CLIで全文を読む。
SessionStartでskillを渡しただけではrules-readyの印を付けず、KDR選択や記録完了も兼ねない。
任意のcatを免除する例外拡大、権限追加、工程state追加は不要である。

既存のM1許可で要求された最小skill・hookの接続を成立させる通常の実装詳細として採用する。
本体testでRule未読時の拒否を維持し、liveのユーザーpromptへ手順を再掲せず、配置された案内から動けることを確認する。
