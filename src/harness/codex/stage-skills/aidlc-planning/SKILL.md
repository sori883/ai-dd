---
name: aidlc-planning
description: planningでメインAIが実行可能な実装方法とUnitを具体化する。
---

要求、既存実装、test、決定を照合し、重大な主張を出典で確認する。結果が変わる未解決判断を示し、通常の実装詳細は根拠を添えて決める。利用者が得る結果を縦に通す単位でUnitを作り、所有file、順序、依存関係、受け入れ条件、実行commandと期待結果、必要な復旧方法を記す。別の担当が会話なしで実行できる本文にする。stage-plannerは工程採否・順序の提案担当のままで、この実装計画を担当しない。計画を作るだけの依頼で実装を始めず、既存の承認経路に従う。

現在の工程・許可担当・Sensor・人間承認に従う。担当追加や子の独自起動をこのskillから許可しない。同じrootのwriterは一人に保つ。共有文書の保存はメインAIが [aidlc-okf](../aidlc-okf/SKILL.md) を読み、aidlc memoryで行う。担当は本文案と根拠を返す。

[原典・変更点](references/source.md)

検証境界やhandoffを具体化するときは [作業方法の詳細](references/handoff.md) を読む。
