# Claude標準の承認質問を対話起動で確認する

2026-09-12。[選択式承認の採用](../decisions/2026-09-12-claude-question-approval-accepted.md)を受けたG0の実測記録。製品hookを実装した結果ではなく、Claude Code 2.1.238の入力・出力の接続条件を確認した結果である。

## 試験条件

macOSで既存のClaude CLIを通常の対話モードで起動した。`-p`、権限を迂回するoption、hookからの回答の自動補完は使っていない。permission modeはdefaultで、画面上はmanual modeだった。今回の起動で新しいtrust dialogは表示されなかったため、初回trust dialogを検証した証拠にはしない。

試験場所は `/Users/const/sori883/ai-dd-validation/three-hosts-20260912/` の `claude-approval-probe` と、新設した `claude-question-probe`。既存のGo標準ライブラリ製 `hook-probe` を再利用し、質問用の後者に設定を追加した。hookは受信JSONを専用eventsへ保存して `{}` を返す。質問への回答を生成するhookではない。

`--setting-sources project --strict-mcp-config --mcp-config '{"mcpServers":{}}' --tools AskUserQuestion` を指定し、架空Intentと架空承認IDへの質問だけを依頼した。外側の試験AIが端末のキー入力で選択・手入力・取消を行った。人間本人が実案件を承認した記録とは区別する。試験会話とイベントの原本は専用環境に留め、RAMへ会話全文を転載しない。

最初の対話プロセスは起動時に2.1.238と表示された。その実行中にCLIが更新を通知し、既定の実行リンクは2.1.269へ変わっていた。後続の取消試験は、既存の `/Users/const/.local/share/claude/versions/2.1.238` を直接指定し、同じ固定版の画面表示を確認した。本作業で更新コマンドや外部toolのインストールは行っていない。

## 観測結果

| ケース | 端末で行った操作 | hookで確認した結果 |
| --- | --- | --- |
| probe-native-001 | Approveを選択 | AskUserQuestionのPre/Postを受信。Postのanswersに質問文をkey、Approveをvalueとして取得 |
| probe-native-002 | Request Changesを選択 | 同様のPostにRequest Changesを取得。前の質問とは別のtool呼出しID |
| probe-native-003 | Type somethingから「まだ判断できません。検証方法を確認したいです。」と入力 | Postのanswersに自由記述を取得。選択肢以外のため承認判断に対応させない |
| probe-native-004 | Escで質問を取消 | 画面に回答拒否、試験会話に同じtool IDのis_error結果が残った。成功Postは受信しなかった。この設定ではFailure hook未登録 |
| probe-native-005 | Type somethingからApproveと手入力 | 選択時と同じanswersの値、空のannotationsとして届いた。クリックと同文の手入力を区別する情報は観測できなかった |
| probe-cancel-006 | PostToolUseFailureも登録した別の設定でEsc取消 | Preの後に成功PostもFailureもStopも受信しなかった。会話には同じtool IDの拒否結果を確認。プロセスは通常の終了操作で閉じた |

最初の5件はsession `b4366860-ffb1-4da8-82f4-f477be1bb3ce`、Failure登録後の取消は `e1e0a000-7671-4c41-8fc1-58cfc0298a82`。どちらも試験終了時に `/exit` で閉じ、終了コード0を確認した。

正常回答のPre/Postでは、`session_id`、`prompt_id`、`tool_use_id` が一致した。回答が質問toolの結果として返り、それだけでは新しいUserPromptSubmitは発生しなかった。したがって、通常入力の更新だけを待って承認回答を保存する接続では不足する。

Preの `tool_input` はquestionsを持つ。Postではtool_inputにもanswersとannotationsが追加され、tool_responseにもquestions・answers・annotationsが入る。Pre/Postの入力JSON全体のbytes一致を要求してはいけない。質問文、選択肢とその順序、multiSelectなど、質問を表す構造を取り出して比較する必要がある。

## 実装契約への反映

承認の条件は、承認対象へ結び付いた質問の回答がApproveまたはRequest Changesと完全一致することとする。「選択式」は物理的なクリックの証明ではない。質問画面内で同じ選択値を手入力した場合も同じ入力として届く。選択肢以外の自由記述や、質問の外の通常チャットは承認に使わない。回答した人物の認証や操作方法の判別を、新しい保証として付け加えない。

現行GoのCaptureApprovalは、通常の入力をその時点の全pending承認IDに結び付け、sessionとturnで再送を検査する。Claudeの一つの質問を一つの対象に限定する要件には、そのまま使えない。質問のtool呼出しIDと対象を一時保存し、decision入口で既存のstateと履歴の保存へ接続する。

取消の復旧では、終了hookの到着を前提にできない。古い質問の対応を明示的に無効化してから新しい質問を提示する手順をD2へ追加する。無効化は承認を作らず、後着した旧tool IDの回答を拒否する。workerの予約解放や実プロセスの終了証明とは別の操作である。時間経過や新しいSubmitだけで自動無効化はしない。

別の計画担当が読み取り専用で確認し、未回答質問だけを対象とした無効化は、採用済みの取消・再提示方針内の実装詳細と整理した。回答保存と無効化を排他的に行い、回答保存後の無効化や別toolの解除を拒否する条件を計画へ追加した。汎用のsession復旧ではBash等も解除し得るため、そのまま流用しない。

## 残る確認

この試験で、正常な質問と回答の対応、選択値、選択肢以外の自由記述、取消時の終了通知欠落が確認できた。質問中のbackground通知、再開・接続断、同じprompt内の複数質問、保存失敗、故意に不正な対応IDを渡したときの製品の拒否は未検証である。これらはD2の回帰testとD3の実機検証に残す。初期化から工程完了までの製品動作、担当起動の制御、配布のtrustを確認した実績には含めない。

承認方式はユーザー確認済みであり、今回の実測結果を理由に再確認しない。初回導入範囲の回答は別に待っている。製品コード・設定・Issue・PRを変更せず、Goのテストも実行していない。
