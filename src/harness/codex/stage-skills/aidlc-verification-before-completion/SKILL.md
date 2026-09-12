---
name: aidlc-verification-before-completion
description: 完了や修正成功を述べる前に、主張に対応する最新の実行証拠を確認する。
---

主張を証明するcommandまたは観測を選び、許可された検証範囲で実行する。出力全体、終了code、失敗件数、対象revisionを確認して結果を述べる。過去の成功、部分検証、担当AIの成功申告だけで全体成功としない。test成功とbuild成功を区別し、要求ごとの証拠を確認する。TDDのRED/GREENは実際の記録を使い、後から正しい実装を戻して証拠を作らない。対象が変更された証拠は更新する。未実行や失敗は理由と残件を明示し、工程の既存Sensor・承認判定を置き換えない。

現在の工程・許可担当・Sensor・人間承認に従う。担当追加や子の独自起動をこのskillから許可しない。同じrootのwriterは一人に保つ。共有文書の保存はメインAIが [aidlc-okf](../aidlc-okf/SKILL.md) を読み、aidlc memoryで行う。担当は本文案と根拠を返す。

[原典・変更点](references/source.md)
