# Issue #130 独立レビュー指摘の一括修正

work_unit_id: four-stage-review-repair。verification_mode: loop。
開始HEAD: 62d8355。workdir: /Users/const/sori883/ai-dd。
許可: 承認済み4ステージ計画の契約違反と受入不足の修正。新しい製品範囲や依存は追加しない。
所有: src/internal/flow、src/internal/minimal、対応CLI test、src/cmd/aidlc/flow*test.go、
src/harness/codex/minimal配布手順、src/core/minimalの関連手順、対応install test、詳細契約とRAM証拠・索引。
単独writerはgo_tdd_implementer。親はhandoff後に対象を編集しない。

## 修正する振る舞い

1. configureはUnitの計画を更新する操作とし、status/result_commit/integrated_commitを任意に生成・変更できないようにする。
   新Unitはpending/空結果だけ。既存Unitの進捗は専用Unit操作が管理する。実行中・結果未回収のUnitを除去しない。
   `go test -count=1 ./src/internal/minimal ./src/internal/flow -run '^TestFlowConfigure'`
2. claim時に依存先の統合commitが後続Unitのbase_commitへ含まれていることをGitで確認する。
   stateがintegratedでも統合前baseを拒否する。
   `go test -count=1 ./src/internal/flow -run '^TestFlowUnit'`
3. review assign/acceptで独立rootの実コード版・bytesを確認し、対象と異なるcheckout・空directoryを拒否する。
   共有Knowledge/ADRは調整rootの明示参照をreviewerへ渡せる。共有文書をreviewrootへ全複製する要求はしない。
   通常運用の版取り違え防止とし、同一OS権限相手の著者認証は追加しない。
   `go test -count=1 ./src/internal/flow -run '^TestFlowReview'`
4. artifactにstate/runtime正本を指定してレビュー結果保存自体でtargetが変わる自己循環を拒否する。
   kindとstageを検査し、現在までのstageのartifactだけ存在/内容検査する。
   将来stageのartifactは計画としてtargetへ含めるが、作成前の段階で存在を要求しない。
   `go test -count=1 ./src/internal/flow -run '^TestFlowSensor'`
5. Unit IDは計画時から単一path componentとして安全な形式へ限定し、空・重複に加え../等を拒否する。
   `go test -count=1 ./src/internal/flow -run '^TestFlowSensor'`
6. live stale拒否は実CLI exit・診断と対象review requestを照合する。hook rawのsubstring検索だけを証拠にしない。
   実モデルのtool結果/JSONLを利用する。hostによる別CLIテストをモデル自身の実行と偽らない。
   実review報告のsession/root/target/status/summaryと、CLIに受理された結果を構造化して照合する。
   reviewer checkoutの対象版も記録し、偽accept・echo文字列・別対象review等でverifierが失敗する回帰testを作る。
   `go test -count=1 ./src/cmd/aidlc -run '^TestFlowCommand'`
   integration付き同Testでharnessのcompile確認をするがlive本体は実行しない。
7. 配布skillの説明・手順を平易な日本語にする。skillのname、CLI token、JSON fieldなど識別子は保持。
   4KiB bootstrapを超える場合は、日本語の短い入口とCLIまたは明示参照で読む配置手順に分け、
   AIが必要な完全手順へ到達する経路と配布testを追加する。読込上限を黙って緩めない。
   `go test -count=1 ./src/internal/install -run '^(TestFlow|TestInstall)'`

## 実行と返却

各振る舞いで回帰testを先に追加し、有効なREDを確認してGREENへ進む。
文書だけの翻訳に人工REDは不要。新たな公開運用を勝手に選ばず、契約内の修正は追加承認を待たない。
関連する既存fixtureを正しいreview checkoutへ修復することも許可する。
末尾に全targeted群と変更影響package test、gofmt、diff checkをまとめて実行。
全project/race/vet/crossbuild/fresh一周/liveは禁止。結果と実行証拠をRAMへ記載しWORK_UNIT_READYで返す。
