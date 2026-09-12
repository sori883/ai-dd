---
name: aidlc-to-spec
description: discoveryで確定した要求をrequirementsの本文案へまとめる。
---

既存の会話、用語、決定、実装を読み、利用者の問題、得られる結果、利用場面、確定した契約、受け入れ条件、観測可能な検証方法をまとめる。新しい面談を自動で始めず、未決定の箇所を事実のように補わない。既存の検証境界を優先し、利用者に見える結果から検証を説明する。Issue trackerへの公開やラベル操作を行わず、requirements向け本文案をメインAIへ返す。

現在の工程・許可担当・Sensor・人間承認に従う。担当追加や子の独自起動をこのskillから許可しない。同じrootのwriterは一人に保つ。共有文書の保存はメインAIが [aidlc-okf](../aidlc-okf/SKILL.md) を読み、aidlc memoryで行う。担当は本文案と根拠を返す。

[原典・変更点](references/source.md)
