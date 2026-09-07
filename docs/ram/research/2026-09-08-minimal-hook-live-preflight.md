# 固定Codexで最小KDR hookの前提を実測する

- 日付: 2026-09-08
- 対象: Issue #128 / M1先頭の技術確認
- 環境: macOS arm64、codex-cli 0.153.4、gpt-6-astra / medium
- 状態: Preflight passed（liveの15 hook入力・8 transport呼出しを補正後の判定器で再検証済み）

## 何を確かめるか

KDRの記録漏れを防ぐには、通常のAI操作前に拒否できることと、進行中の操作が終わるまで
記録済みと判定しないことが必要になる。文書上のschemaだけでなく、freshなGit利用先で
Bash/apply_patchと実際のhook通知を突き合わせた。

Go test実行ファイルを専用hookとして使い、外部module・別runtimeは追加しない。
既存のHOME/CODEX_HOME、認証、保存済みtrustは変更していない。
検査済み試験hookに対する一回限りのtrust bypassとworkspace-write、approval neverを使用した。
user hookは事前に内容確認済みのEdit/Write用助言と通知音であり、試験専用の観測ログとは区別した。
transcriptのturn_contextでworkspace-write・network_access=falseと、.git/.agents/.codexのread保護を確認した。

## 設定読込で判明したこと

最初の2回はproject hookが読まれず、禁止canaryも作成された。成功とは扱わなかった。
macOSの /var/folders は /private/var/folders として正規化し、起動先・信頼対象・設定に同じ実pathを使う。
さらに `-c projects."<root>".trust_level=...` は固定版のCLI parserで引用符までキーに含まれる。
modelを呼ばないapp-server config/readで実効値を確認し、map全体を渡す
`-c 'projects={"<root>"={trust_level="trusted"}}'` に修正すると正確なrootキーになった。
3回目のliveではproject hookが実際に発火した。

## 実際に受けた通知

- SessionStartはsource=startupでturn_idなし。UserPromptSubmitはturn_idあり。
- PreToolUseの拒否により `touch probe-forbidden` は実行されず、canaryは存在しなかった。
- 通常成功・exit 7の失敗の双方で、同じtool_use_idのPre/Postが届いた。
- 3秒待機する成功・失敗の各commandは、初回応答でsession_idを返し、write_stdinで完了した。
  初回の非同期返却を終端Postにせず、完了時に同じtool_use_idのPostが一件届いた。
  各Postを受けた時点で、command末尾に書くファイル本文も存在した。
- apply_patchのPre/Postにも同じtool_use_idがあり、tool_input.commandにpatch本文が入った。
- Stopは最初stop_hook_active=false、一度decision:blockを返した後trueで再入した。今回turn_idは同じだった。

重要な形式差はBashのtool_responseが実出力本文だけで、終了コードを含まなかったこと。
試験でstdoutへ出力しないcommandのPostでは空文字になった。
製品は成功・失敗の両方を未記録のまま扱うため、exit codeの推定は不要。
実行中slotの解除根拠は対応するPostイベントであり、出力中の「終了した」という文字列ではない。
read-only technical_researcherもraw証跡を照合した。拒否されたPreにはPostが届かないため、
slotの取得は入力・KDR・Rule等の検査を通って実行を許可する経路だけで行う。拒否パスでslotを残さない。

試験のtranscriptではcode-modeのcustom_tool_call/execとJSON形式の実行結果が記録された。
外側のcall_idはhookのtool_use_idとは別の識別子なので、同一視しない。
成功/失敗code、初期session_idとpollの対応を試験内だけで確認し、製品hookにtranscript解析を要求しない。
この試験ログは製品の全操作auditとして配布・保存するものではない。

## 証拠と限界

3回目の試験保存先は
`/var/folders/9w/921pjkys39q28sk4xsc0hs000000gn/T/aidlc-minimal-hook-probe-2082657890`。
raw入力はevent-*.json、AI操作はstdout.jsonl、試験用の輸送確認はtranscript.jsonl。
試験の最初の判定器は「Post本文に終了コードがある」という誤った仮定で失敗した。
判定器を実入力に合わせて補正し、欠落・早期Post等を拒否する条件を維持して再検証する。

補正後、`AIDLC_MINIMAL_HOOK_EVIDENCE=<上記保存先> go test -v -count=1 ./src/cmd/aidlc -run '^TestMinimalHookProbeReplay$'`
で同じraw証拠の検証が成功した。`go test -count=1 ./src/cmd/aidlc -run '^TestMinimalHookProbe'`も成功。
親も両コマンドを再実行し、read-only技術調査の確認と合わせて本体実装へ進めると判定した。
独立reviewerも固定した2試験fileのhashとraw証跡を照合し、同じReplay・targeted testを確認した。
本体着手を妨げるblocking findingはなかった。これはpreflightの限定レビューであり、本体実装のレビュー完了を意味しない。
補正のための新しいliveモデル呼出しは行わず、失敗した初回判定を成功だったことに書き換えない。

この結果は固定版と上記直列操作の実証であり、全Codex版、全tool、故障時の常時強制を保証しない。
compact・強制中断・実際のKDR保存との接続はM1の後続試験で確認する。
