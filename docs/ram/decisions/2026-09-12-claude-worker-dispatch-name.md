# Claudeの担当名の長さをアダプターで吸収する

2026-09-12。Issue #185、承認済みD2の実機補修。ユーザーの「core実装ではなくCodex／Claude Codeへ配布するアダプター」という指定を維持する。

candidate-02の固定Claude 2.1.238では、予約済みworkerのtask_nameが71文字で、Agent.nameの64文字上限に拒否された。nativeの入力検証errorに `^[A-Za-z0-9][A-Za-z0-9_-]{0,63}$` と上限が表示された。製品Preより前の拒否であり、worker起動成功ではない。nameを省略した別要求も製品Preが予約不一致として拒否した。

共通task_nameは `aidlc_<epoch32hex>_<assignment ID32hex>` のまま保つ。Claude側は二つの16byteをつなぎ、小文字・paddingなしbase32へ変換し、aidlc_を付けた58文字を起動用名として渡す。Pre/Postで逆変換すれば元の予約名へ完全に戻る。追加のalias台帳、候補検索、自然文からの推測を作らない。Go標準ライブラリで実装する。

公開assignment応答には `dispatch_name`（native起動のnameへ渡す値）を追加する。共通appは名前投影portを呼ぶだけで、具体変換はharnessに置く。Codexはtask_nameと同じ値、Claudeは58文字を返す。保存済みtask_name、予約ID、registry schema、子のnative ID、SendMessageの宛先は変えない。Unitありでもassignmentのlist/showで起動名を取得できるよう案内する。

worker用aliasは長さ・文字・decoded 32byte・再encode一致を検査し、不正な値を素通ししない。異なるepoch／IDは既存の予約照合で拒否する。読取り担当の自由なnameはnativeの既知の文字・長さ範囲で検査する。結果提出やStopによる予約解放は追加しない。

読取り専用の計画担当が確認した。共通生成名を短縮する代案は、Claudeの制約のためにCodexと保存契約まで変えるため採用しない。承認済みのworker起動を成立させる環境別変換の具体化であり、追加承認を待たず同じ単独writerでTDD修復する。

実機根拠はclaude-tdd-02、native session `52b244ac-00f3-4b58-888a-189933e2a269`、拒否tool `toolu_01GJfovVb6ZxNNjzPwPfmF7V`。候補binaryは[初回G0記録](../research/2026-09-12-claude-product-g0-first-run.md)のcandidate-02。試験sessionは正常終了した。修復後は新規配置と予約でworker実起動を再検証する。
