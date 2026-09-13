# Claude製品アダプターの実機G0と一時ロック競合

2026-09-12。Issue #185のD2実装を、利用案件と分離した新規配置で試験した。固定Claude Code 2.1.238を通常のmanual permissionで起動し、製品設定とhookをそのまま使用した。core資材は変更していない。

## 対象と確認できたこと

candidate-01のbinary SHA256は `ad93d3d16557fcfd28e30c62931fd38c296b80ec0b41e71ff576ecabb98d04d7`。sourceのHEAD・115変更fileの集合は[loop返却記録](../decisions/2026-09-12-claude-adapter-boundary-repair.md)を参照する。配置設定SHA256は `771d55dbdb88ace2cc26a199b5aa41a61639c95c71857fbb75aac7fa06595987`。

通常TUIでRule再読と現在工程取得が実行できた。initializationで禁止されたworkerの実際のAgent呼出しは、製品Preが `agent is not allowed in current stage` と拒否した。許可されたstage-plannerは構造化 `name` を渡して起動し、子のPreを通ってseed.mdを読み、本文を返した。descriptionやprompt本文にnameを書いた最初の要求は拒否された。自然文から識別子を推測する代替は採用しない。

再試行の `run_in_background` は、AIがbooleanではなく文字列 `"false"` として送り、nativeは非同期起動した。この結果は同期起動成功の証拠にしない。後述する3番目の候補ではfield省略でも非同期になったため、文字列だけを非同期化の原因とする当初の解釈は撤回する。nameが通常TUIでは使えないという断定も行わない。

## 起動通知が重なる問題

Agent Postが13:11:40.997 UTCにassignmentsのロック競合で停止し、SubagentStartは13:11:41.014に成功した。保存済みdispatchは `start_seen=true / post_seen=false`。後の読取りではロックは残存していなかった。子起動と読取りは成功したが、この候補のG0全体は合格にしない。

読取り専用の計画担当が、hookからの担当更新だけ、ロック取得を既存session hookと同じ2秒上限・20ms間隔で待つ案を確認した。取得後に最新registryを読み、判断と保存を一度だけ実施する。ロック削除、イベント全体の再実行、保存失敗の自動再試行は行わない。期限超過時は競合未解消として停止し、記録と予約を保持する。通常CLIの即時競合エラーは維持する。

これは承認済みD2の正常イベント順序差への補修であり、担当制限やworker予約を弱めない。単独writerへ回帰testからの修復を依頼し、新候補で再測定する。

同じ末尾確認で、Claudeの入口skillが置換前3936 bytesあり、長いbinary pathでは共通4 KiB上限を超えることが分かった。配布skillを整理して詳細を既存のCLI skillへ移し、実行pathが長い新規配置のSessionStartを回帰確認する。上限自体は緩めない。

## 証拠の保管と未完了gate

試験rootは `/Users/const/sori883/ai-dd-validation/three-hosts-20260912/product-g0/claude-init-01`。native sessionは `933cee7e-417a-490b-8caf-c4bb82900291`。同階の `claude-init-01-debug.log` と、このroot専用のClaude transcriptを照合した。準備用Sensor/reviewは公開CLIによる合成fixtureであり、実案件の独立reviewではない。

質問の実回答、再開、worker競合、Codexの接続回帰、修復後G0、独立review、read-only final、PR checksはそれぞれ別の確認として記録する。

## 初回候補の質問実機結果

同じ通常TUIで、実際の質問回答からAIが自分のDraftへ生JSONを保存し、公開 `intent approval` で登録した。Request Changesはrevision 4→5でrejected、Approveは別requestのrevision 7→8でapprovedとなり、質問のanswer_key・選択値・answer_hashがstateのturn・quote・prompt_hashへ一致した。通常Submitが挟まっても、先の質問回答を使って登録できた。

Esc取消は質問 `toolu_01SzZap9hmBtAhhzVZ7DuJMg` がpreparedのまま残った。試験コントローラーが公開show/abandonでその質問だけを失効し、別tool IDで同じrequestへ再提示できた。自由文Holdはno_answerとなり、承認stateはpendingを維持した。これも明示失効した後、新質問でApproveを選べた。取消・自由文を承認へ読み替えていない。未観測の古いUIへの後着回答は、決定論的testの拒否証拠と区別する。

質問証拠はcandidate-01/question-evidenceへ保存した。登録操作の途中、AIはDraftをMarkdownの作業記録だと解釈し、別の/tmpへ判断JSONを作ろうとしてPreに拒否された。自分のDraftは拡張子.mdでもCLI用の生JSONに使える旨を、次候補のClaude CLI skillへ明記した。native sessionは正常exit 0で終了し、次工程へ進めていない。

## 修復後の候補

2項目の回帰testはそれぞれRED→GREEN。親の対象確認もexit 0だった。117変更fileのmanifest SHAは `a315ff058290990ff4d2964127f90458b12f258e35c56e364644db31ecd81fc2`。candidate-02 binary SHA256は `546b0c2e363c570baf4ce8367677c9792d03f34485d0f2350b80fb7f253ae4d6`。変更は取得待機と配布案内で、coreとmoduleは不変更。

新規のClaude TDD fixtureとCodex initialization fixtureへ配置した。Codex 0.153.4では通常のproject trust、hook画面で5製品hookのcommand確認と信頼を実行し、全5件のActiveを確認した。既存global hookは変更せず、更新案内はSkipした。Codexの別execとClaudeのworker起動・再開は、この時点では実行中である。

## 2番目の候補の確定結果

Codexの別execはexit 0で終了した。実際のspawnで禁止workerを拒否し、許可されたstage-plannerがseedを読んだ。同じ子のcanonical名へのfollowupで再読取りと `G0-CODEX-RESUMED` の返却まで確認した。native sessionは `01a095d1-c183-7d20-ba39-be205deb1380`、製品の対応はbound、post_seen=true。この接続の回帰成功であり、全工程の実案件完走ではない。candidate-02のcodex-summary.json、codex-native-relevant.jsonl、codex-registry.jsonへ証拠を保存した。

ClaudeのTDD fixtureでは、予約名71文字がnative Agent.nameの64文字上限により、製品Preより前で拒否された。子は起動していない。CLI予約とnative起動がつながらない製品接続の問題として[可逆の起動名](../decisions/2026-09-12-claude-worker-dispatch-name.md)を採用した。両環境のプロセスを正常exit 0で閉じ、停止通知からworker予約を自動解放していない。

## 3番目の候補

起動名の修復は3項目で意図したRED→GREEN、公開CLIとnative変換の接続fixtureはALREADY_GREEN。親の関連5package確認はexit 0。124変更fileのmanifest SHA256は `121c5609270bc10962b8d9e1705a039c065f8af35b48801cf24476f90e833c46`、candidate-03 binaryは `c83f175bb868516367ac9d291c38f66ad8c6d1fbe3b497fe9dcd4785eb124d50`。core資材、module、予約の保存名・schemaは変更していない。

新しいclaude-tdd-03へ公開CLIと明記した合成回答でTDDを準備した。準備scriptの応答field名の誤りでnative UUIDを保存できなかった最初の予約は、子・処理が一度も起動していないことを確認し、公開releaseで明示解放した。この試験準備の失敗を製品REDに含めない。次のUUIDと予約を保存してから実Claudeを起動した。別の調整sessionから同rootへ予約する公開CLI対照は `worker root is already reserved` と拒否し、registryが不変だった。

58文字のdispatch_nameを構造化nameとして渡した実Agent `toolu_01W6ndqwDm6GfHWeTJyA1kQV` は起動に成功した。子 `ad47cb7de695cee10` はRead一回でseed本文とG0-WORKER-READを返した。13:53:28.774 UTCのStartと同28.797のPostはともに成功し、共通dispatchのstart_seen/post_seenはtrue、予約名は元の71文字のままboundになった。同35.645のStop後もboundを保持し、自動解放しなかった。子・親の実tool履歴とregistryをcandidate-03へ保存した。モデルの最後の説明にある「reserved」は古いshowの値であり、実記録はboundである。

次に同じ子へのSendMessage、重複worker起動、禁止担当起動を依頼したが、Claudeが `Credit balance too low` を返した。追加のnative toolは一件も実行されていない。この候補での追加依頼、同役割連続起動、未知の子・古い工程、同期順序は未検証。課金・購入・認証変更を行わず、TUIを正常exit 0で閉じた。固定binaryのversionを再確認し、2.1.238のままだった。

## 同期指定の案内訂正と再開条件

3番目の実Agent入力にはrun_in_backgroundが存在しないが、nativeは非同期起動した。調査担当と親が確認した[公式subagent仕様](https://code.claude.com/docs/en/sub-agents#run-subagents-in-foreground-or-background)は、interactiveでfork modeが既定の場合、Agentはbackgroundとなり同期指定を受けないと説明する。[fork modeの説明](https://code.claude.com/docs/en/sub-agents#turn-fork-mode-on-or-off)は2.1.232以降を対象としている。現行公式文書と固定2.1.238の実測が整合するという根拠であり、文書全体が固定snapshotだとは扱わない。nameとは別の仕組みで、製品adapterが非同期を強制したわけではない。

Claude配布の入口・CLI skillからfalseを指定する例を取り除き、公開tool schemaに従い完了通知を待つ説明へ修正した。変更は2つのMarkdownだけ。現在124fileのmanifest SHA256は `2d4d3e7d6baa8448b15b06253d56ddf63671824f15fc3ccd515886f36ea69f59`。candidate-03の実機はこの文章修正前の資材なので、両者を同一の配布物と称さない。共通Goやnative処理には追加変更がない。

Claudeの残高が回復した後、専用fixtureの同じ会話と子を使って追加依頼・再開の実測を続ける。同期の実機gateには、通常TUIの既定動作と区別した試験プロセスだけの `CLAUDE_CODE_DISABLE_BACKGROUND_TASKS=1` が公式に示されているが未実施。製品利用の必須設定へ追加していない。実機G0の残件を完了するまで、独立review、read-only final、PR作成・mergeは未完了として保持する。Issue #185はopen、製品差分と本記録はローカル未commitである。
