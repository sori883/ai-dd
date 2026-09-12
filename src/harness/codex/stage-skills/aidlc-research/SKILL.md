---
name: aidlc-research
description: researcherが仕様やAPIの事実を一次資料で調査する。
---

問い、対象version、確認範囲を受け取る。公式文書、仕様、実装、第一者APIを根拠にし、各主張をその事実を所有する出典へ結ぶ。確認できた事実、推論、矛盾、未確認事項を分ける。再現手順や引用箇所を添えた一つのMarkdown本文案をメインAIへ返す。背景調査が必要な場合の担当起動はメインAIの既存native起動経路を使い、researcher自身が子を起動しない。

現在の工程・許可担当・Sensor・人間承認に従う。担当追加や子の独自起動をこのskillから許可しない。同じrootのwriterは一人に保つ。共有文書の保存はメインAIが [aidlc-okf](../aidlc-okf/SKILL.md) を読み、aidlc memoryで行う。担当は本文案と根拠を返す。

[原典・変更点](references/source.md)
