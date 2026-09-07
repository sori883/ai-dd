# M1実機検証で見つかった編集失敗と復旧の確認事項

状態: 確認待ち。Issue #128、検証対象 `3fb1cbcbe40fa2b11fc6f7369efb1d4cb7a58cd0`。
これは実機検証の失敗記録であり、M1完了や新しい運用の承認ではない。

## 確認したこと

Codex CLI 0.153.4 / macOS arm64 / gpt-6-astra / medium、通常のworkspace-writeとapproval neverで実行した。
独立レビューの指摘修正後にread-only finalを開始した。
vet、format、module tidy差分、6構成のcross-build、既存と新方式の非live統合試験は成功した。
全package testとraceは、旧Space構成・以前のhelp全文を期待する既存testの更新漏れで失敗した。
その期待値は新仕様に直接対応する修正対象とし、製品を旧仕様へ戻す理由にはしない。

通常一周のliveはexit 1。実テストのRED→実装変更→GREENと、二会話で同じKDRの選択・更新は確認した。
ただし第二会話がKDR更新後にGitの追加確認を行い、未記録へ戻った後に再更新しなかった。
Stopは一度補完を要求し、再入時は警告して終了した。clean終了の受入条件を満たさず、成功扱いしない。

質問・故障のliveでは、質問待ちのKDR保存後、同じ会話への回答でdraftのDelete/Addを含む
`apply_patch`が失敗した。PreToolUseで取得した実行中IDに対するPostToolUseがなく、IDが残った。
その後の一般操作は拒否された。これは保存済みKDRを失ったという意味ではない。
境界liveの最終結果もexit 1。質問待ち、Rule欠落、CLI故障、別会話再開の証拠は得られたが、
同一会話の回答記録と残留IDからの復旧が成立せず、実行中IDが一件残った。二つのliveとも未合格である。

実機証拠は以下の一時directoryに保存した。利用プロジェクトの成果物をこのRAMへ転載しない。

- 通常一周: `/private/var/folders/9w/921pjkys39q28sk4xsc0hs000000gn/T/aidlc-minimal-journey-3400645237`
- 境界試験: `/private/var/folders/9w/921pjkys39q28sk4xsc0hs000000gn/T/aidlc-minimal-boundaries-2139006976`
- 対象編集ID: `exec-09ff1e6d-2d86-4169-b1b8-26d32b8c78de`

## 固定版の根拠

公式tag `rust-v0.153.4` のcommitは `3d2ee51ca2d5db578f328aa75e20aa22c0197c9a`。
[`registry.rs`](https://github.com/openai/codex/blob/3d2ee51ca2d5db578f328aa75e20aa22c0197c9a/codex-rs/core/src/tools/registry.rs#L662-L718)
は成功のToolOutputがある場合だけPostToolUseを呼ぶ。失敗は内部のlifecycleでは終端になるが、hookへは通知しない。
[`apply_patch.rs`](https://github.com/openai/codex/blob/3d2ee51ca2d5db578f328aa75e20aa22c0197c9a/codex-rs/core/src/tools/handlers/apply_patch.rs#L390-L456)
はparse/verify失敗をFunctionCallErrorとして返す。固定版にはPostToolUseFailure相当のイベントもない。
前のpreflightで確認したBashのexit 7・非同期poll完了・成功patchを、失敗patch全体の保証へ広げられない。

## 確認する具体案

同じIntentへ明示的に復旧する既存の `session bind <id> --space <space> --session <session> --recover`
を、編集失敗で終了通知がない場合にも使う。AIはtoolから返された失敗と実行が残っていないことを確認する。
IDの手入力を利用者へ求めず、AIが同じ会話・Space・Intentを指定する。

hookはその固定CLIの単独呼出しだけを復旧例外として通し、既存Serviceが実行中IDを解除する。
他のIntentへ切り替えること、一般操作を例外にすること、自動で終了を推測することは認めない。
未記録は保持し、KDRと必須Ruleを読み直して、失敗・復旧・検証・残件を通常updateで記録する。
現hookの例外判定は実行中IDが空であることを要求しており、既存の明示recoverを妨げるため修正が必要。

承認後の対象は `src/internal/minimal/hook.go` と回帰test、配置skillの正確な復旧手順、
実機試験・利用手順・計画とRAM。外部module、工程state、全操作audit、独自receiptは追加しない。
失敗patchから同IDで復旧できること、他session/Intentや通常操作は引き続き拒否されること、
復旧だけでは記録済みにならないことをtest-firstで確認する。独立review後に全finalと二つのliveを再実行する。

既存計画は残留IDの明示recoverを許可しているが、[M0契約](../../design/minimal-product-contract-m0.md)は
「固定版のhook入力から終端を識別できなければ…実装を止めて対応方法を確認する」と明記する。
このため今回判明した失敗patchへの適用を無断で採用せず、上記の対応方法をユーザーへ確認する。
