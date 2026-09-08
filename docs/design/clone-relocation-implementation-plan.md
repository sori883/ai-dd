# 別cloneへの配置移転とUnit再割当を実装する

状態: Accepted。[提示案](clone-relocation-proposal.md)と「この方式で実装を進めてよいですか」に
ユーザーが「はい」と回答した。直接承認として実装する。基準mainは82b711f7cbabfc4b1d9473d156cad7dc7d4e0988。

## 利用者が得る結果

現在はGitでstate・Knowledge・ADRを移せるが、配置hookの旧絶対pathとローカル担当割当が残り、
新しいcloneでAI作業を続けられない。本変更では移転先を明示して製品参照を更新し、停止確認済みの
Unitを同じID・成果のまま新担当へ割り当てる。旧処理を遠隔停止したとは扱わない。

## 公開操作

```sh
aidlc install codex --relocate --project-dir NEW_ROOT --from-project-dir OLD_ROOT --from-binary OLD_BINARY
aidlc unit reassign INTENT_ID --space SPACE --expect REVISION --file reassignment.json
```

reassignment.jsonは次のobject。run_idは新規発行するので入力しない。

```json
{"unit":"unit-a","session":"new-worker","root":"/new/worker","commit":"40桁の現在HEAD","reason":"旧処理終了と引継ぎ成果を確認","previous_run_stopped":true}
```

新rootは実在するGit worktreeの絶対root。旧root/binaryは移転元が存在しなくても扱い、参照文字列としてのみ
検査する。存在確認や旧repositoryへの書込みをしない。移転先AI会話を開始する前に端末からrelocateを実行する。
新binaryは実行中のaidlc自身。旧path省略・通常installとのflag混用・重複flagを拒否する。

## 配置移転の保存契約

変更対象は既知の `.agents/skills/aidlc/SKILL.md` のbinary参照と `.codex/hooks.json` の製品command文字列のみ。
Skillは現在の埋込原稿を旧/新binaryで展開した確認済みbytesと一致するときだけ更新する。
今回SKILL原稿は変更せず、手順追記はWORKFLOW原稿とhelpへ行う。既存WORKFLOWの更新をrelocateに含めない。

hooksは5 eventの既知製品handlerを一意に識別し、型・既知属性・必須eventを検査する。
JSONの重複key、不正UTF-8、不正構造、末尾data、過大data、symlinkを拒否する。
更新は確認済みcommandのJSON文字列spanだけ。独自hook・未知keyや書式を含む他のbytesを保持する。
未知の製品編集や識別不成立は全保存前に拒否し、対象pathと不一致理由を表示する。
各hook/Skillが確認済み旧参照または新参照のいずれかなら、途中の混在でも同じ操作を再試行できる。
旧=new root/binaryのケースを含め、既に更新済みなら変更なしで成功する。

全対象を読み取り検査後、移転先の専用lockを取得/保持し、各fileの元bytesを保存直前に再確認して原子的に保存する。
部分成功時には更新済みpath、未処理path、診断を返す。次回は全件を再検査する。新しいtransaction台帳は作らない。
通常AI運用での競合防止であり、同一OS利用者がlockを無視した全書込み経路を禁止するものではない。
Knowledge・ADR・Rule・state、認証/model設定、ユーザーhookを変更しない。
新rootのCodex hook trustはユーザー管理の外部設定。変更せず、新hooks.jsonの絶対pathと確認手順をhelpに示す。

## Unit再割当の保存契約

active Intentのtdd、対象Unitのneeds_confirmationだけを受理する。
runningのまま移転した場合は既存pause/resumeで確認待ちにし、CLI成功まで新workerを開始しない。
停止確認true、理由、新session/root、現在commitを必須とし、実HEAD一致、baseからの履歴、依存統合、
scope逸脱と別worker root/sessionの衝突を検査する。新run IDを発行しUnitをrunningへ戻す。
ID・計画・基準・成果commitを保持する。旧run IDのresultを拒否し、再テスト後に新runでresult/integrateする。

他Unitがrunningならassignment必須。needs_confirmationなら不存在だけはroot/session比較を省略できるが、
scopeと依存検査は常に維持する。破損・読取失敗は不存在と同一視せず拒否する。
対象Unit自身は他担当の衝突比較から除外するので、runtimeのない複数Unitを順に再割当できる。

Store.changeの排他/CAS内で検査→新runtime保存→state保存。
runtime失敗ならstate不変。state失敗ならneeds_confirmationを保持しresultを拒否する。
残存割当が同じ再割当要求なら同じ新run IDでstate保存を再試行する。
従来claim/confirmによる旧割当は停止確認に基づき置換できる。reassign途中の異なる残存要求は黙って上書きせず、
現在割当の再確認を案内する。stateが既にrunningなら再割当せずshowとassignmentから成功済みを確認する。
reasonと停止確認は現行割当の属性として保持し、audit履歴や新工程stateを追加しない。
移転先では既存review assignで新root/sessionへ割当し現在targetで受け直す。

## 実装所有と順序付きTDD

単独Go実装担当、work_unit_id=clone-relocation、verification_mode=loop。以下を1 Issue/PRで順に行う。
外部module追加なし。標準ライブラリと既存依存のみ。必要な内部helper抽出は同じ意味を維持する。

1. install: 新relocate.go/test、必要ならinstall.go内の既知資産生成helper抽出。旧/新判別・独自bytes保持・不正拒否・競合/途中失敗/再試行。
   `go test -count=1 ./src/internal/install -run '^TestRelocate'`
   新APIは `Relocate(root,binary,fromRoot,fromBinary string)` と結果型をtest可能にするcompile-only宣言をRED前に許可。
2. flow: unit.goと新reassign_test.go、必要時内部helper。停止確認・履歴/依存/scope・複数Unit・旧run拒否・runtime/state失敗再試行。
   `go test -count=1 ./src/internal/flow -run '^TestFlowUnitReassign'`
   UnitRequestへReason/PreviousRunStoppedのfield追加はcompile-onlyとして許可する。
3. CLI: minimal.go/help.go/cli.goとtest。relocate flags、reassign操作、型/値/JSON例/部分成功/停止確認の説明。
   `go test -count=1 ./src/internal/cli -run '^TestRelocationCLI'`
4. 接続: minimal/command.go/flow.go/hook.goとtest。既存の同session/Intent/tool slot境界でreassignを認識。
   移転操作を古いhook経由の自動修復例外にはしない。
   `go test -count=1 ./src/internal/minimal -run '^TestRelocation'`
5. cmd: relocation_integration_test.goで新旧rootの実CLI一周。複数Unit引継ぎ→実test→result/integrate→新review。
   移転元のstate/設定hash不変、独自hook/Rule/Knowledge/ADRの保全、更新後hookが新rootだけを使うこと。
   `go test -tags=integration -count=1 ./src/cmd/aidlc -run '^TestRelocationCommand'`
6. WORKFLOW原稿、docs/development.md、four-stage詳細契約、RAM証拠と索引を更新。旧検証で記録した制約は削除せず後続記録を付ける。

各sliceはtest先行RED→GREEN、既成立の期待はALREADY_GREENとして正直に記録する。
末尾で上記5targetedと既存install/flow/unit/help/hook関連prefix、gofmt/diffを確認し親へ返す。
親は差分/targeted末尾を一度確認し、独立review・修正後のread-only finalを実施する。

## 最終検証

全test、race/shuffle、vet、tidy-diff、format/diff、全integration、CGO無効の6構成buildとnative help。
固定Codex 0.153.4/gpt-6-astra mediumの限定liveは、移転済みの検査済みhook/Skillで同じIntentを選択し、
新rootのKnowledge作成/更新と元root無変更を確認する。実AI新runのUnit完走は上記実CLI fixtureと区別する。
入口案は `AIDLC_RELOCATION_LIVE=1 go test -tags=integration -v -count=1 -timeout=15m ./src/cmd/aidlc -run '^TestRelocationLive$'`。
既存live helperを再利用し、trustは固定検証で使用中の一時CLI設定だけを使用しユーザー設定fileを変えない。
停止・失敗・証拠不足は成功とせず、推測したモデル完了文を検証に使わない。

## 本家準拠と許可境界

固定AI-DLC 2.6.123のmanifest/emit/guideとroot解決を確認済み。配置先と新root基準は一致する。
本家の通常配置はcopy中心、同名relocate/reassign契約は確認されず、この2操作は承認済みGo四段階製品の追加機能。
本家install/updateと同等とは主張しない。Go単一binary・利用者編集資産保持の既承認境界を維持する。
本家hook trustは絶対pathに結び付くため、移転先でのユーザー側の信頼確認を案内する。

Issue分類は機能開発。独立review・final・対象PRのGitHub CI成功後に通常merge commitで自律マージする。
新たな重大な運用選択や承認範囲外が判明した場合だけ確認へ戻る。
ユーザーAGENTS差分・参照資料・他worktreeを保全。ローカルの製品コードはGitで戻せるが、利用先の
担当を巻き戻す操作は自動化せず、同じ停止確認と現物確認を必要とする。

## 独立レビューで確認した契約修復

20156f0の独立reviewで、途中保存後にconfigureや別Unit操作がrevisionを進めると、
未完了reassignを異なる要求で上書きできる問題を再現した。既存の「同要求だけ再試行」契約を守るため、
未完了reassignが残る当該Intentでは、同要求を完了するまで他のstate更新を既存lock内で拒否する。
公開configureが通るStore.Saveとchangeの双方を対象にする。新state/schema/履歴は追加しない。
またGitの引用済みpathがscope比較を壊すため、reassign/resultのpath一覧をNUL区切りの生bytesで扱う。
日本語・空白・改行を保全し、scope外は引き続き拒否する。

この修復は承認済み受入条件違反の修正として追加承認なしで行う。
repair work unitはclone-relocation-review-repair、loop、単独writer。
所有範囲をflow/{reassign,unit,store,review,sensor}.goと関連test、既存手順/helpとRAMへ限定する。
P1は保存失敗→configure/別Unit拒否→同要求同run復旧→後日の再割当を、P2は追跡/未追跡の日本語・空白・改行pathと
resultまでをtest先行で確認する。正確なtargetedは
`go test -count=1 ./src/internal/flow -run '^TestFlowUnit(Reassign|Result)'`。
末尾に元の5targetedを再実行し、再review後に未着手のfinalへ進む。
