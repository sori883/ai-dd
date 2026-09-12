---
name: aidlc-domain-modeling
description: discoveryで用語、具体例、重要な判断の意味を揃える。
---

既存の共有知識にある用語と発言を照合する。同じ語が別の概念を指すなら区別を提案する。関係を正常例・境界例・例外で確かめ、定義、別名、具体例、判断と理由、採用しなかった選択肢を本文案へ反映する。確定事項と未確定事項を区別する。CONTEXT.mdや独自ADRを別の正本として作らず、OKFの共有文書へ統合する案を返す。

現在の工程・許可担当・Sensor・人間承認に従う。担当追加や子の独自起動をこのskillから許可しない。同じrootのwriterは一人に保つ。共有文書の保存はメインAIが [aidlc-okf](../aidlc-okf/SKILL.md) を読み、aidlc memoryで行う。担当は本文案と根拠を返す。

[原典・変更点](references/source.md)
