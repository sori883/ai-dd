# 作業記録をOKF検索対象にする実装計画

## 背景と利用結果

現在はステージの差戻し理由をIntent（ひとつの目的を持つ作業）のフォルダ内work-log.mdへ保存するため、SpaceのOKF検索で見つけられない。利用者が理由を読み返せるよう、作業記録をOKF MarkdownとしてKnowledgeへ保存する。OKFはMarkdown先頭のfrontmatterで文書の種類や検索用情報を揃える形式である。

## 許可と契約

ユーザーが提示計画に対して保存先をknowledge/log/へ補正し、その確認後に「はい」と承認した。これは本変更への直接承認であり、旧33 Stageの包括承認を流用しない。[承認RAM](../ram/decisions/2026-09-09-work-log-okf-request.md)が根拠である。

- 保存先: `aidlc/spaces/<space>/knowledge/log/<intent_id>-work-log.md`。
- 初回のintent reopenで生成し、その後の差戻しは同じファイルへ追記する。本文は従来どおり日時・元/先ステージ・revision・理由を扱い、全操作日誌へ拡張しない。
- frontmatterはtype: work-log、intent_id、Intent名を含むtitle、差戻し記録を示すdescription、tags: [work-log]、generated.by: process:aidlc、generated.atをCLIが生成する。statusは既存生成規則のstable。generated.atは内容更新時のUTC時刻であり、人間の承認や検証済みを意味しない。
- 通常のmemory searchによるtitle/description/tags検索とintent_idの完全一致絞り込み、memory showでの本文参照を使用する。本文全文検索を追加しない。
- state.jsonはIntent配下を維持。ファイルの存在だけで差戻し成功とせず、stateのrevision確定条件を維持する。
- 新規Intent向け。既存ファイルの削除・自動移動・移行・二重書込みはしない。旧pending保存形式を誤って継続しないよう検証する。旧配置を自動更新しない。

## 保存の具体化

現在の空ファイルの仮作成は、Knowledgeへ置くとOKF検索を壊す。そのため初回の土台も正しいfrontmatterを持つ文書として原子的に作成する。既存OKF serializerとmetadata生成を再利用し、本文とmetadataを一緒に保存する。追記時は未知のmetadataを保持する。

pending（保存途中の状態）には追記前と完成後の文書hashを記録する。初回要求で時刻を固定し、再試行で日時を生成し直さない。現在ファイルが追記前なら完成版を保存し、完成後なら追記せずstate確定だけを再試行する。どちらでもない改変・欠落・別要求を拒否する。256 KiBの上限はfrontmatterを含む実際の完成bytesで事前検査する。不正文書、別Intent/type、symlinkや特殊ファイルを安全に拒否する。

これはOKF v0.2の任意typeと標準generatedを利用する製品側の保存契約変更である。固定AI-DLC 2.6.123の33段階と全操作auditを復活させる変更ではなく、既に承認された4段階の差戻し記録の配置を変更する。

## 所有範囲とTDD

1 Issue/PRの全項目をwork_unit_id: okf-work-logとして単独のgo_tdd_implementerへ渡す。親は計画・Issue・レビュー・最終検証・PRを管理し、実装中に対象を編集しない。

1. 保存先とOKF metadata・追記・検索: src/internal/flow/reopen_log.goと同packageのwork_log_okf_test.go、必要に応じsrc/internal/okfmemoryの既存serializer利用。`go test -count=1 ./src/internal/flow -run '^TestOKFWorkLogDocument'`。
2. 保存途中と再試行: reopen_log.go、store.goのpending検証、reopen_log_test.go、新しいwork_log_okf_test.go。`go test -count=1 ./src/internal/flow -run '^TestOKFWorkLogRecovery'`。各保存境界の失敗、初回/既存記録、改変・欠落、上限超過、異なる要求を確認する。
3. 公開CLI・配布手順: src/internal/minimalのwork_log_test.go、src/internal/cli/help.goと対応test、src/core/workflow/stages/*.md、src/harness/codex/minimal/WORKFLOW.md、必要な配布参照test。`go test -count=1 ./src/internal/minimal ./src/internal/cli -run '^TestOKFWorkLog'`。検索のSpace/Intent絞り込みとshow、helpの保存先を確認する。
4. src/cmd/aidlc/flow_journey_integration_test.goの旧保存先期待を変更し、実CLIで新記録のOKF検索を検証する。これは配布結合検証なので実行はfinalへ集約し、loopで配布E2Eを繰り返さない。

各Go動作はテスト先行で期待する失敗を確認してから最小実装する。後続が既に成立した場合はALREADY_GREENと記録する。既存reopen回帰テストの保存先期待のみ必要に応じ更新する。追加API scaffoldは不要。作業単位末尾に上の3targetedコマンドと `go test -count=1 ./src/internal/flow ./src/internal/minimal ./src/internal/cli ./src/internal/okfmemory ./src/internal/install`、gofmt適用、git diff --checkを行う。RAMに実測証拠を記録し索引を更新する。

## 受入条件と最終検証

新配置で差戻し理由が一度だけ保存され、検索で正しいIntentの記録が見つかり、showで本文を読める。別Space/Intentが混ざらない。途中保存後にもOKF検索が壊れず、state確定前の記録を成功扱いしない。保存失敗後の再試行・既存の差戻しgateを保つ。旧ファイルは変更しない。手順・helpは新pathを示す。

独立reviewでblocking findingがなくなった固定HEADに対し、親がread-only finalを1回行う。`go test -shuffle=on ./...`、`go test -race -shuffle=on ./...`、`go vet ./...`、`go mod tidy -diff`、`gofmt -l src`、`git diff --check`、`go test -tags=integration -count=1 ./...`、darwin/linux/windowsのamd64/arm64の6build、native help/version/reopen helpを確認する。外部Go moduleと外部toolは追加しない。対象HEADのGitHub checks成功後に通常のmerge commit方式で取り込み、Issue closeとmain反映を確認する。

リスクはmetadata更新によるhash変化と複数ファイルの保存途中状態であり、固定時刻・前後hash・失敗注入testで検証する。ロールバックは新規利用を止めて元版を利用し、作成済み記録を削除・変換しない。中断中の人間承認実装には着手しない。
