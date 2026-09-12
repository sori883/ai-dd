---
name: aidlc-architecture
description: architecture-analysisで実装済みの構成を根拠から説明する。
---

entrypoint、設定、schema、主要な処理とtestを辿る。正本、依存方向、system境界、権限境界を先に説明し、重要な処理を入力から検証、保存、副作用、応答、失敗と回復まで実行順に記す。部品の所有責任と根拠fileを対応させる。図は境界や流れを明確にするときだけ使う。既存文書も事実確認の対象とし、将来案を実装済みと書かない。共有current-analysis/architecture向け本文案と根拠、未確認事項を返す。

現在の工程・許可担当・Sensor・人間承認に従う。担当追加や子の独自起動をこのskillから許可しない。同じrootのwriterは一人に保つ。共有文書の保存はメインAIが [aidlc-okf](../aidlc-okf/SKILL.md) を読み、aidlc memoryで行う。担当は本文案と根拠を返す。

[原典・変更点](references/source.md)
