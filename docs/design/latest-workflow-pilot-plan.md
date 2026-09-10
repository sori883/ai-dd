# 最新構成で実案件を完走する計画

## 目的と現在地

AI-DLCは、Goの単一実行ファイル、6種類のステージ、5種類の専門担当、OKF文書、
開始・終了Sensor、独立レビュー、会話での人間承認を備える。
基準はmainのPR #162、commit `62429d4f644293093ef4e5ae2fcc3ec5a24b8c8b`。実施管理はIssue #163。
個別テストと担当起動の実機検証は済んでいるが、この組合せで実際の改善案件を完了した証拠はまだない。

実案件は `intent check --help` の案内改善とする。現在は開始・終了検査の使い方を説明するだけで、
どの文書型と本文見出しが必要かをhelpだけで把握できない。6段階の既定の開始条件・終了条件と、
現在Intentの具体的な入出力を `intent procedure` で確認する方法を日本語で示す。
旧 `check-help-pilot-plan.md` の4段階・advance・WORKFLOWを前提とした実行案を、この計画で置き換える。

ユーザーは2026-09-10に「①最新版で実案件を完走、②配布・更新、③利用者文書」の説明に対し、
「じゃまず一番から順次対応してもらえますか」と依頼した。これを3項目を順に進める直接依頼とする。
小さなhelp改善の対象選択は①を具体化する通常の実装判断であり、新しい製品仕様は追加しない。
この作業開始の許可と、製品が各ステージで要求する成果承認は区別する。
後から作られる承認要求へ、この依頼を過去回答として転用しない。

## 作業場所と保存先

- 製品開発branch: `codex/latest-workflow-pilot`。
- 調整用worktree: `/Users/const/sori883/ai-dd-latest-pilot`。
- 元の `/Users/const/sori883/ai-dd` の未コミット資料は変更しない。
- 実案件のSpace名: `workflow-pilot`。新規IntentをCLIで作る。
- 利用データは専用worktreeの `aidlc/spaces/workflow-pilot/` に置き、製品のPRへ含めない。
- CLIバイナリと実測ログは専用worktreeのGit対象外 `aidlc/evidence/latest-pilot/` に保存する。
- 永続する判断と実測結果の要約だけを本開発の `docs/ram/` に記録する。

## 順序と人間承認

1. **初期化**: 基準版をbuildし、fresh install、Space・Intent作成、Rule読込、procedure取得、
   開始Sensor、begin、終了Sensorを実CLIで確認する。独立レビューの実報告を受け取り、
   初期化成果の承認要求を生成してユーザーへ提示する。
2. **目的整理と深掘り**: 調査と要件整理を別担当へ依頼し、要件を保存する。
   stage-plannerが6段階を使う実行計画を提案し、ユーザーが計画とdiscovery成果を承認する。
3. **現状の構成分析**: 現行help・Sensor・CLI・配布手順を解析し、Space共有の
   `codekb/current-analysis.md` と `codekb/architecture.md` を作る。
4. **実装計画**: 変更fileとTDD順序を `design/<intent_id>/implementation-plan.md` に具体化する。
5. **TDD**: 登録した別worktreeの1workerへ1作業単位を依頼し、test-firstでhelpを改善する。
6. **統合検証**: 成果を統合したcommitで検証し、現行の使い方をKnowledgeへ保存する。
   終了Sensor、独立レビュー、ユーザーの成果承認、finishを経て同じIntentのcompletedを確認する。

各回に開始Sensorとbegin、終了Sensor、独立レビュー、提示後の実回答による成果承認が必要。
計画変更にも新しい承認が必要。承認待ちの間に後続の一般作業を先行しない。
ADRはアーキテクチャ判断を変更するときだけ作る。help説明だけの変更なら不要理由をreviewする。
差戻しが発生したら製品のreopenを使い、理由を `knowledge/log/<intent_id>-work-log.md` に保存する。

## 実機の経路と未確認条件

現会話で使える開発用agentと、配布する `aidlc-*` の5担当は別物である。
開発用agentの独立レビューは実レビューとして記録できるが、製品のnative担当起動の実測には数えない。
製品5担当と自動hookの実行は、配置先を読んだCodex 0.153.4の通常の会話で観測する。
必要なら既存Codex CLIをその会話の実行環境として用いる。aidlc CLIが担当を起動する機能は追加しない。

実行前に配置したhookの絶対pathと内容を提示し、通常のCodex hook信頼確認を通す。
既存試験fixtureの `--dangerously-bypass-hook-trust` を実利用へ流用しない。
初回のassignment initは人間が既知作業を確認した回答を根拠とする。read-only担当のnative起動も
registryを読むため、最初の製品reviewerより先にこの確認が必要。
本会話の実回答を別のCodex実行環境へ中継する場合は原文と出典を保持し、実際に受信したhookの
session/turnを使う。試験用の架空回答や直接state編集で人間承認を代替しない。
通常の信頼確認や回答中継を実行できない場合、初期化で止めて必要な操作を具体的に提示する。
自動hookや製品担当を観測していない段階では、最新版の実機完走を主張しない。

## 変更範囲と単独writer

| 対象 | 変更 | 所有者 |
| --- | --- | --- |
| `src/internal/cli/help.go` | check helpの6段階・文書型・本文見出し・証拠と承認の説明 | Go実装担当1名 |
| `src/internal/cli/check_help_test.go` | 公開help出力の回帰testを新設 | 同上 |
| `docs/design/latest-workflow-pilot-plan.md` | 実行契約と検証範囲 | 親AI |
| `docs/ram/decisions/2026-09-10-sequential-completion-request.md` | 今回の依頼と順序・境界 | 親AI |
| `docs/ram/decisions/2026-09-10-latest-workflow-pilot-progress.md` | 実際の到達点、証拠、未完了gate | 親AI |
| `docs/ram/README.md` | 記録の索引 | 親AI |

親AIは共有state/Knowledgeのwriter。子は資料・本文案・成果を返し、共有記録を直接更新しない。
コード変更は別worktreeの単独writerに任せ、同じworktreeで同時編集しない。
並列worker自体の実機検証はPR #162にある。この小さなhelp変更は競合を増やさず1workerで扱う。

## TDDと受入条件

Go作業単位は `latest-check-help`、verification_modeは `loop`。
全sliceを1回のhandoffで渡し、各項目のrunnable RED→最小GREEN→refactorを順に行う。
対象commandは `go test -count=1 ./src/internal/cli -run '^TestCheckHelp'`。

1. check helpで6段階の開始/終了、Rule、要件、構成分析文書、実装計画、必要時のADR、
   宣言文書、TDD/integrationの実測証拠が分かることを公開CLI出力で検査する。
2. Requirements/ImplementationPlanのintent_id、各型の必須見出し、共有文書の扱い、
   planningを省略した場合と文書outputsなしの扱いが現在のSensor契約と一致することを検査する。
3. checkがread-onlyで、passだけで完了しないこと、begin・review・approval・finishと
   procedureへの案内があり、helpの既存呼出し形式とstdout/stderr/exit codeが維持されることを検査する。

Sensorの判定、state形式、コマンド引数、承認やhook保証を変更しない。
新しい外部Go moduleや外部toolを追加しない。

## レビューと最終検証

開発用の独立reviewは `verification_mode=review` とし、固定diffと対象Sensorの整合を確認する。
blocking finding修正後に親がread-onlyの `final` を1回開始する。

- `go test -shuffle=on ./...`
- `go test -race -shuffle=on ./...`
- `go vet ./...`
- `go mod tidy -diff`
- `gofmt -l src`、`git diff --check`
- `go test -tags=integration -count=1 ./src/internal/workspace ./src/internal/okf`
- `go test -tags=integration -count=1 ./src/cmd/aidlc -run '^(TestFlowJourney|TestIntentDocumentsJourney|TestAssignmentJourney)$'`
- darwin/linux/windows × amd64/arm64の6buildとnative helpの確認。
- 実案件の全実行回のSensor・review・承認・履歴、OKF検索、最終completedを現物で照合。

固定Codexでの今回の実案件の観測を使い、同じ目的の長時間fixtureを理由なく重ねない。
最終検証後の変更は証拠を古くするため、必要なloop/reviewを経てfinalを取り直す。
PRはIssueへ紐付け、必要なchecks成功後に通常のmerge方式でmainへ取り込み、Issueのcloseを確認する。

## ②配布・更新と③文書への引継ぎ

①の結果を受けて②、次に③へ進む。②では各OS向け配布物と検証値、既設の保全・比較・更新・復旧を
具体計画にする。公開version、既存保存dataや利用者編集をどう扱うかに未確定の重大な選択がある場合は、
完成した候補と影響を提示して確認する。今回の依頼を既存data削除・認証やtrust無効化の許可へ広げない。
③ではREADME、導入と初回利用、各役割と保存先を現行動作へ合わせる。

固定本家AI-DLC 2.6.123について確認済みの配布・Space・メインAIによる担当起動を基準とし、
最新upstream全体との一致は主張しない。①は既存契約の説明・実利用確認であり、新しい意図的差分はない。
実案件環境の中断・復旧ではstateと予約を保持する。失敗時に削除して未実行へ見せかけない。
