# 製品4担当の実装証拠

Issue #140、work_unit_id=product-agent-roles、verification_mode=loop。
承認は[直接依頼](2026-09-08-product-four-agents-approved.md)、範囲は[実装計画](../../design/product-agent-roles-plan.md)。

researcher・requirements・workerを追加し、reviewerの独立検査と調査への返却責任を整理した。
4定義は利用者のmodel/effortを継承する。読取り3担当とworkspace-write workerを分け、共有stateと
Knowledge/ADRは調整役だけが内容確認して保存する。WORKFLOW、Rule、開発手順を整合させた。
既存の動的列挙で配置されるためGo本体とSKILL原稿は変更していない。

## 実測

- TestProductAgentAssetsを先に追加。`go test -count=1 ./src/internal/install -run '^TestProductAgent'` は
  定義数1対4と新3定義の不存在でexit 1（runnable RED）。ログ `/tmp/product-agents-red.log`。
- 原稿追加後の同commandはexit 0（GREEN）。ログ `/tmp/product-agents-green.log`。
- Python3標準tomllibで4fileを解析。名前/path一致と重複なし、必須文字列、sandbox_mode、model/effort未固定を確認しexit 0。
  定義・手順の文言自体には人工的なREDを作っていない。
- 末尾 `go test -count=1 ./src/internal/install -run '^(TestProductAgent|TestFlowInstall|TestInstall|TestRelocate)'`
  はexit 0。ログ `/tmp/product-agents-boundary.log`。新Go testへgofmt、git diff --checkを実施。

既存利用先への自動更新は行わない。4定義と新手順の配置が利用条件となる。
固定Codex0.153.4で4 named agentの実spawnは親のfinalへ残し、このloopでは未観測。
配置とTOML検査を実model起動や任意の調査品質の保証とは扱わない。

## finalで検出した旧fixtureの修復

TestMainSpaceCreateClosedPipesが複製していた旧Rule全文と、承認済み新配布原稿が不一致だった。
これはINVALID_TEST_FIXTUREであり製品のREDとは数えない。
`go test -count=1 ./src/cmd/aidlc -run '^TestMainSpaceCreateClosedPipes$'` でexit 1を再現後、
Ruleの期待bytesだけをcore/minimalのembed原稿から取得し、同commandでexit 0を確認した。
ログは `/tmp/product-agents-pipe-before.log` と `/tmp/product-agents-pipe-after.log`。
file残存、完全bytes一致、権限・時刻を含む再実行不変のassertは維持し、Go本体は変更していない。
gofmtとgit diff --checkを実施した。親の全体finalは修正後に再確認が必要。

## 親の独立レビューと実起動確認

2b0377efa68f12f595ac008adb62aa0ea7871cb5を独立reviewし、blocking findingなし。
旧Rule複製の修復を含む保存失敗・配置targetedも親がexit 0を確認した。

同じ版でread-only finalの全Go test、race/shuffle、vet、tidy-diff、format/diff、全integrationが成功。
darwin/linux/windowsのamd64/arm64向け6構成buildとnative build/installも成功した。
配布した4定義をPython標準tomllibで解析し、必須項目・名前・sandbox・model/effort継承と原稿bytes一致を確認。

固定Codex CLI 0.153.4、gpt-6-astra / mediumで4担当を各1回実起動し、すべて終了した。
一時fresh配布先の検証専用SubagentStart/Stop hookから、4種類のagent_typeと異なる4個のagent_id、
対応する終了・返答を照合した。全員が同じ資材の確認印を読み、researcherは事実/不明点、requirementsは
受入条件案、workerはtest案、reviewerは曖昧点を返した。配布定義と資材bytesは不変だった。
証拠は `/private/tmp/ai-dd-product-agent-final-20260908-203008/` のresults.json、agent-static.json、
live-summary.json、agent-events、live-last.txt。再現手順は実装計画のfinal欄。

このsmokeは親のread-only設定で全担当を起動した名前解決・読取り・返答の確認であり、
workerの書込み、実案件のTDD品質、製品hookを通した4段階全体の保証とは区別する。
テスト専用rootのhookだけを置換し、ユーザーの既存設定・認証・配置先は変更していない。

本追記は上記実装版の観測結果。追記後の固定HEADについて再度read-only finalを行い、
最終のhead、検証結果、GitHub checksとmerge結果はIssue #140に紐づくPRへ記録する。
