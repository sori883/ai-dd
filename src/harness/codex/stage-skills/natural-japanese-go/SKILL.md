---
name: natural-japanese-go
description: 日本語文書の作成・推敲で構成と読みやすさを確認し、Kagome版CLIの検出結果を判断材料にする。
---

読者、目的、伝える結論を先に確認する。[文章の設計と推敲](references/writing.md) を読み、素材に基づいて本文を作る。診断のみの依頼では書き換えず、根拠と改善候補を返す。

下書きをUTF-8ファイルへ保存したら、PATH上の `natural-japanese-go --json FILE` で通常14カテゴリを検査する。文章種別が明確なら `--genre essay|tech|business` を使う。使い方と対象は `natural-japanese-go --help` と `--list-rules` で確認する。Python、uv、実行時辞書downloadは不要。

検出ごとに文脈を読み、「修正する」「理由を示して残す」「素材が不足する」を決める。指摘数を減らすために数値や体験を創作したり、自然な反復を機械的に削ったりしない。再検査では前回JSONを別ファイルに保持し、`natural-japanese-go --json --baseline PREVIOUS.json FILE` で新規・継続・解消を確かめる。新しい指摘がなく、残す理由と素材不足が説明でき、見出しと段落先頭の通読で論旨が通れば仕上げる。

検査は本文を変更せず、指摘があってもexit 0。exit 1は入出力・解析・baselineの失敗、exit 2は引数不正。失敗を検出なしと報告しない。Go版同士だけでbaseline比較し、Sudachi版との完全一致、著者判定、自然度点数を約束しない。実験検査・reading-load・semanticは別機能で、このCLIでは拒否する。outlineとtermsの代わりに見出しの論旨と用語の一貫性を自分で読む。

AI-DLCでは開始済み工程と既存hookの検査に従って実行する。承認待ちの例外を作らない。担当は本文案・検出結果・判断をメインAIへ返し、共有文書の保存はメインAIが [aidlc-okf](../aidlc-okf/SKILL.md) を使う。同じrootのwriterは一人に保つ。

[原典・対応範囲](references/source.md)
