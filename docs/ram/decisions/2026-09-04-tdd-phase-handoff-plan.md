# Go TDDの依頼をREDとGREENへ分離する計画

- 日付: 2026-09-04
- 状態: Accepted（ユーザーが提案済みの依頼単位分割を直接承認）
- 基点: 6f33c82067298ba9732f4c3f828c2b79a9b15a46
- Issue: [#94](https://github.com/sori883/ai-dd/issues/94)
- 実装: 親が運用ファイルの単独writer。動作評価は隔離したGo課題で別agentが担当する。

## 背景・目的

Goの開発では、先に期待する動作のtestを書いて失敗を確認するRED、そのtestを最小実装で成功させる
GREENを繰り返す。しかしIssue #93では、1回の依頼で機能一式を任せたため、最初のRED/GREEN後に
本体実装が先行し、残りのtestが後追いになった。途中報告を求めても、親が結果を確認する前に
作業を継続できたことが、検出を遅らせた。

この変更では「test作成・失敗確認で一度終了」「親が再確認」「その1件だけ実装して終了」という
依頼単位に変更する。中間連絡toolやcommentaryの到着を進行条件に使わず、完了した返却と実測を使う。
後続の開発者は、各機能がどの失敗を直したかと、誰が次段階を許可したかを追えるようになる。

## 実装許可と境界

ユーザーへ上記の分割を提案した後、ユーザーは「まずその手順の仕組みを実装」「ファイルを直す」
「依頼の単位を分けるところを確実にやりたい」と明示した。これを二段階の開発運用を整備する
直接承認とする。AI-DLC製品機能の既存マイルストーンを、運用変更の許可へ流用しない。

RAMの圧縮・削除・再編はユーザーが見送ったため行わない。必要な今回の合意記録と索引追加だけを行う。
Issue #93の未採用実装は別branchに保ち、修正・採用・mergeをこの承認に含めない。
agentのmodel、推論強度、sandbox、追加権限、外部module/tool、製品CLIや保存形式は変更しない。

本変更は、親が次の依頼を発行する前の検証手順であり、OSの書込権限やCodex本体のtool実行を
制限する新しいsecurity機構ではない。文章を書くだけで絶対に違反不能と主張しない。
本家AI-DLCの製品挙動を変えないため、新しい意図的な本家差分はない。

## 実装する契約

- verification_mode=loop/review/finalは維持する。Goのloop依頼には別軸のtdd_phase=redまたはgreenと、
  1件の観測可能な振る舞いを識別するslice_idを必須にする。欠落・不明・複数振る舞いの一括依頼は停止する。
- REDは指定testだけを追加・変更して最小のtargeted commandで失敗を観測し、最終応答で終了する。
  コンパイル不能なAPIの場合は、親が事前に明示した型・signature・空の返値等の骨組みだけを例外にできる。
  自分で必要範囲を拡張しない。compile failure、環境failure、testが実行されない結果はREDの証拠にしない。
- 初回GREENならALREADY_GREENとして返し、完成実装の削除やtestの歪曲によるREDを作らない。
- 親は返却後に変更対象、test内容、骨組み以外の本体実装がないことを確認し、同じtargeted commandを
  再実行する。意図した期待値不一致を確認した後だけ、同じsliceのGREEN依頼を発行する。
- GREEN依頼には、親が確認したREDのcommand・失敗内容と、Git HEADおよび対象fileのSHA-256
  （未作成fileはABSENT）を含むred_acceptanceを明示する。
  root、slice、対象file、command、内容が変われば古い受入を使わず停止する。
- GREENは受入済みtestと対象scopeを変えず、1件を通す最小実装、同じtestの成功、必要な範囲のrefactor、
  gofmt後のtargeted確認だけを行って最終応答で終了する。次のtestや次sliceを開始しない。
- 親が成功結果を再実行して確認してから次sliceを渡す。test誤りや別仕様が見つかれば親へ返し、
  testの期待値をGREEN側で緩めない。
- 既存の単独writer、独立review、read-only final、現在headのGitHub checks成功というgateを維持する。
  個々のRED/GREEN返却で全検証を繰り返さない。

## ファイルと単独所有権

親が次の運用ファイルを編集する。詳細はdocs/tdd-handoff.mdへ集約し、重複した長文指示を増やさない。

- AGENTS.md: 親が各phaseの結果を受け取り、確認前に次を発行しない必須ルール。
- docs/agent-workflow.md: 標準フロー、役割、handoffの接続。
- docs/tdd-handoff.md: 入出力、RED/GREEN境界、親の再実行、snapshot、失敗時の扱いの正本。
- .agents/skills/go-tdd/SKILL.md: 1依頼1phaseで終了する実装担当の手順。
- .codex/agents/go-tdd-implementer.toml: 同じphase条件をagent起動時にも適用。
- .agents/skills/implementation-planning/SKILL.md: 1振る舞いのsliceとphase別対象を計画へ含める。
- .agents/skills/go-code-review/SKILL.md: phase返却・親の受入・freshな証拠をreviewで確認。
- 本計画、docs/ram/README.md: 合意と検証結果を記録。

## 検証と受け入れ条件

運用ファイルの更新なので、製品Goコードを変更するTDDそのものではなく、skillのforward-testを行う。
隔離した一時directoryに、標準ライブラリだけの小さなGo課題と実装前の骨組みを用意する。
実際のIssueを根拠に、この課題だけのtestと最小実装を評価agentへ許可する。製品treeは編集させない。

1. phase指定なし、またはRED受入なしのGREEN依頼ではfileを変更せず停止する。
2. 有効RED依頼はtestを書いて意図した期待値不一致を観測し、骨組みの本体を実装せず終了する。
3. 親がfile差分・hashと同じtestの失敗を確認した後、受入付きGREENでその1件だけ成功させる。
4. testまたは対象snapshotが変更された古い受入は、GREENを開始せず拒否する。
5. 初回GREENの条件ではALREADY_GREENとして返し、人工REDや余計な本体変更を行わない。
6. 親のtargeted再実行と変更file確認で、返却内容と実際の結果が一致する。

forward-testと必要な修正後に差分を固定し、独立reviewを行う。
その後、親がread-only finalとして変更skillのquick_validate.py、TOML parser、
codex features listによる設定ロード、file参照解決、git diff --checkを実行する。
製品Goコード未変更のため、ローカル全package/race/vet/cross compile/製品配布E2Eは繰り返さない。
GitHubでは既存CIのpush/PR全checks成功を確認してから既存方式でmergeし、対応Issue closeを確認する。
final後に対象fileを変えた場合は検証をstaleとし、固定差分reviewと必要なfinalを更新する。

## リスク・戻し方

往復数は増えるが、1件のtargeted testへ絞り、全検証はfinalへ集約する。
途中連絡が使えなくても、最終応答で各依頼を終えるため親が進行を管理できる。
証拠不足や想定外の本体変更は自動修復や上書きで隠さず停止する。
既存の永続dataは移行不要で、問題時は運用ファイルを通常の修正PRで直せる。
検証で分かった不備だけを追加修正し、一般的な巨大な指示集へ広げない。

## 設定の根拠

既存agent設定のname/description/developer_instructionsとmodel等を維持し、developer_instructionsだけを
変更する。公式OpenAI文書のcustom agent設定形式を確認した。
https://learn.chatgpt.com/docs/agent-configuration/subagents#custom-agents

## 動作評価で見つかった問題と修正

隔離したGo課題で、初版の運用指示だけでは次の見落としが実際に起きた。

- phase指定のない依頼を担当がREDと補完した。現在の親メッセージから明示値を取り出し、
  なければ編集もtest実行もしない条件を入口へ移した。別の担当ではBLOCKED・無変更を確認した。
- 受入snapshot内のPLAN.mdが変わっていたが、担当が表示されたhashを一致と誤認してGREENへ進んだ。
  全entryを親の期待値とcheck commandで機械比較し、全件の終了code 0を編集前に必要とする手順へ変更した。
  文書や既存のuser変更も除外せず、不一致を無害と判断して通さない。

これらの失敗を成功証拠へ置き換えず、新しい隔離fixtureで変更後の挙動を再確認する。
失敗したfixtureの実装を削除して、過去にREDがあったように作り直すことはしない。

## 修正後の動作評価結果（2026-09-05）

実際のgo_tdd_implementerを使い、親が返却後のfile内容とtargeted commandを確認した。

| 条件 | 結果 |
| --- | --- |
| phase未指定 | BLOCKED。test未作成・本体無変更 |
| RED受入なしのGREEN | BLOCKED。file無変更 |
| 日本語名を返す1件のRED | testだけを作成してRED_READY。本体は空返値のまま |
| 親のRED再実行 | 指定testが期待値不一致で失敗、終了code 1 |
| PLAN.md変更後に古い受入を渡す | 全4fileのhash機械比較が終了code 1。BLOCKED・本体無変更 |
| 親が同じREDを再確認して受入更新 | 全hash一致後、greeting.goだけを変更してGREEN_READY |
| 親のGREEN再実行 | testは受入時と同一。指定test成功、終了code 0 |
| 既に正しい日本語名のtest確認 | ALREADY_GREEN。人工RED・追加変更なし |

RED/GREENでは同じcommandを使用した。
`GOTOOLCHAIN=go1.26.8 go test -count=1 -run '^TestGreetingJapaneseName$' .`
この評価は依頼分離と停止条件の動作例であり、全ての将来の依頼で指示違反が不可能な証明ではない。
固定差分の独立review、finalおよびGitHub checksの結果は対応PRへ記録する。
