# M6実装結果: CIの実行責任の整理

Issue #214、`work_unit_id=test-reduction-m6`、`verification_mode=loop`。承認済み[M6計画](test-reduction-m6-plan.md)と[直接承認](../ram/decisions/2026-09-14-test-reduction-approved.md)に従った。開始HEADは `63b1263f22de7f7e9bae36153dd9273c96f53dda`、M5 merge baseは `d8aa6cf3415e3cfe984dc2f530bd9da398cdddc2`。本作業ではcommitせず、終了HEADも同じ。既存差分は開始時に無かった。

## 6候補の処置

| ID | 処置 | 生存保証・責任 | loop証拠 |
|---|---|---|---|
| W01 | CIのcross-build jobを削除 | Distribution/Package、固定Go1.26.4の5CLI×6target・CGO=0/trimpath・7assetを維持。Qualityは各Go版native compile/test | Distribution差分はtrigger1行だけ。配布入口discovery成功 |
| W02 | format/vet/race/module/journey/dev smokeへ主要版ifを追加 | Qualityの1.26.x/stable両matrixとfail-fast:falseを保持。通常・filesystem testは両版、計測目的のraceと明示full vetは主要版 | shell構文とjourney2入口discovery成功。実suiteは親final |
| W03 | 仮注入aidlcの別buildと表示検査を削除 | 主要版の1buildでhelp、dev/unknown版、未知引数exit2・stdout/stderrを維持。実注入5CLIはDistribution Native | 前後smoke shellのbash -n成功。smoke実行は親final |
| W04 | 両workflowのpushをmainだけへ限定 | PR・main反映後・手動Distributionを保持。PR未作成branch pushでは自動起動しない | trigger差分確認。実イベントは同head CIとmerge後mainで親が確認 |
| W05 | exact workspace/okfの通常testを同版integrationへ集約 | normal25＋filesystem2＝全27package。各exact対象1回・normal非空・go list失敗時停止。coverage-normal.out/coverage-filesystem.outを分離 | 前後selector実行と集合cmp成功。両packageのproduction集合はtag有無で一致 |
| W06 | 理由付き保持 | 入口照合、同run artifact ID、source/remote tag/main、7file、draft直前照合。3OS Native・Windows PS5.1/7 | Distributionはtrigger以外を変更せず、配布/Bootstrap各2入口discovery成功 |

既存設定の整理としてALREADY_GREEN扱いとし、人工REDやYAML写しtestは作成していない。実testはloopで実行しておらず、discoveryをsuite成功とは扱わない。

## 検証証拠とGo版

localは `go version go1.26.4 darwin/arm64`。Qualityの設定値は文字列 `1.26.x` と `stable`、Distributionは `1.26.4` のまま。CI上のresolved版の確認は親が同head runで行う。1.26.xは厳密な最小patch版検査ではない。

正確な各command・phase・exit・全出力は `/tmp/m6-evidence.json`。package選択のみを実行する `/tmp/ai-dd-m6-package-check.sh` はgo test行を含まず、選択結果と元一覧をsort/cmpする。実workflowから抽出した全package stepはbash -nだけを行う。前後のshellも `/tmp/m6-smoke-before.sh` と `/tmp/m6-smoke-after.sh` に保存した。

| command/診断 | 結果 |
|---|---|
| `go version` | go1.26.4 darwin/arm64 |
| `bash -n /tmp/ai-dd-m6-package-check.sh` / `bash /tmp/ai-dd-m6-package-check.sh` | exit0。normal25、filesystem2、workspace1、okf1。全27集合一致 |
| `go list -f '{{.ImportPath}} {{.GoFiles}} {{.CgoFiles}} {{.SFiles}}' ./src/internal/workspace ./src/internal/okf` | exit0 |
| `go list -tags=integration -f '{{.ImportPath}} {{.GoFiles}} {{.CgoFiles}} {{.SFiles}}' ./src/internal/workspace ./src/internal/okf` | exit0。通常と同一 |
| `go list -tags=integration -f '{{.ImportPath}} {{.TestGoFiles}} {{.XTestGoFiles}}' ./src/internal/workspace ./src/internal/okf` | exit0。追加はworkspace4・okf1のintegration test file |
| `go test -tags=integration -list '^Test(FlowJourney\|GitIndependentJourney)$' ./src/cmd/aidlc` | exit0、各入口exact1件 |
| `go test -tags=integration -list '^TestReleaseCandidate(Metadata\|Native)$' ./src/cmd/aidlc-dist` | exit0、各入口exact1件 |
| `go test -tags=integration -list '^TestBootstrap(PowerShell\|CandidateNative)$' ./src/bootstrap` | exit0、各入口exact1件 |
| `bash -n /tmp/m6-smoke-before.sh` / `bash -n /tmp/m6-smoke-after.sh` | exit0。実build/実行なし |
| `git diff --check` | exit0 |

製品Go/test/module/Action SHA/cache/runner/timeout/権限は変更していない。Go変更がないためgofmt適用対象なし。変更fileとSHA256は `/tmp/m6-file-hashes.json` に保存する。workflow2file、development手順、本結果、承認RAM/索引、milestones末尾の実施状況、および親が確認したM5 main成功を追記するM5完了RAMが変更対象。

## loop返却時点の確認範囲

独立review・親read-only final・同head両QualityとDistribution・merge/main確認は未完。全suite/race/vet/module/journey/dev smoke・Native・30build・実Codex・tag/draft操作はloopで実行していない。M5 main checksは親から初回成功の確認を受け、M5完了RAMへ追記した。現在の限定診断に失敗・skip・未解決事項はない。

## 最終検証とマージ結果の保存先

この文書は単独writerの実装結果とloopの証拠を記録する。親はこの後、独立review、read-only final、現在headのCI/Distributionを確認し、結果と実行リンクを[PR #215](https://github.com/sori883/ai-dd/pull/215)本文へ追記する。mainへのmergeとIssue closeは、そのPRの状態・merge commitで確認できる。古いheadや入口確認だけを最終成功として扱わない。
