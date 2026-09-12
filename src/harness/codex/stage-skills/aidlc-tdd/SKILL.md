---
name: aidlc-tdd
description: tddでworkerが観測可能な失敗testから実装する。
---

許可されたUnitと検証境界を確認する。利用者に見える契約をtestへ書き、実行して意図したassertionの失敗を観測する。compile失敗、skip、未実行はREDではない。必要最小限の実装で同じtestをGREENにして次の動作へ進む。全testを先に書いて全実装を後にする進め方を避ける。内部methodや内部collaboratorの呼出回数に依存せず、外部境界だけを必要に応じて代替する。既存実装で成功したtestはALREADY_GREENと記録する。承認範囲を超えるtest境界が必要ならメインAIへ返す。

現在の工程・許可担当・Sensor・人間承認に従う。担当追加や子の独自起動をこのskillから許可しない。同じrootのwriterは一人に保つ。共有文書の保存はメインAIが [aidlc-okf](../aidlc-okf/SKILL.md) を読み、aidlc memoryで行う。担当は本文案と根拠を返す。

[原典・変更点](references/source.md)

検証境界やhandoffを具体化するときは [作業方法の詳細](references/testing.md) を読む。
