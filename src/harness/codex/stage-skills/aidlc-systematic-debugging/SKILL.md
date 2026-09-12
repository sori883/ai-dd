---
name: aidlc-systematic-debugging
description: 予期しない動作やtest失敗を再現し、原因を検証する。
---

まずerror全文、再現手順、直近差分、入力と出力を調べる。複数componentでは境界ごとにdataと設定の伝播を追い、失敗箇所を絞る。動く例と参照実装を比較し、差を説明できる仮説を一つ立て、一度に一条件で試す。原因が分かる前に修正を重ねない。修正は [aidlc-tdd](../aidlc-tdd/SKILL.md) の回帰testから進め、[aidlc-verification-before-completion](../aidlc-verification-before-completion/SKILL.md) で証拠を確認する。三度の修正でも異なる結合問題が続くなら設計判断をメインAIへ返す。再現できない環境問題は調査済み範囲と不足証拠を報告する。

現在の工程・許可担当・Sensor・人間承認に従う。担当追加や子の独自起動をこのskillから許可しない。同じrootのwriterは一人に保つ。共有文書の保存はメインAIが [aidlc-okf](../aidlc-okf/SKILL.md) を読み、aidlc memoryで行う。担当は本文案と根拠を返す。

[原典・変更点](references/source.md)
