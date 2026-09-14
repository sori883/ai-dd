# M6: CIの実行責任をまとめる詳細計画

GitHub Actionsでは、作業branchへのpushとPR、二つのGo版、配布workflowの間で同じ検査を繰り返している。通常testのGo版互換性、race、実ファイルシステム、配布候補の検査を残し、それぞれを実行する場所と回数を明確にする。

作業先は `/Users/const/sori883/ai-dd-release` のみ。実装baseはM5 merge後の `d8aa6cf3415e3cfe984dc2f530bd9da398cdddc2`。[Issue #214](https://github.com/sori883/ai-dd/issues/214)が作業単位である。許可は `docs/ram/decisions/2026-09-14-test-reduction-approved.md` の直接依頼と `docs/design/test-reduction-milestones.md` のM6。1 Issue/PR・1 writer・`work_unit_id=test-reduction-m6`、writerは `verification_mode=loop`。元案は読み取り専用planningで作成した。親がM5のmerge・Issue closeと生存入口を確認して本稿を確定し、Issue #214を作成した。

## 前提・所有file

- 現moduleは `github.com/sori883/ai-dd`、go.modは `go 1.26.0`。Qualityは `1.26.x` と `stable`、Distributionは固定 `1.26.4`。別名が同じ実Go版に解決されてもmatrixを一つへ減らさない。1.26.xを厳密な最小patch版の検査とは説明しない。
- workspace/okfのproductionには現在build tagがなく、integration tagはtest fileだけ。M4/M5 merge後にも確認する。tagでproductionが変わっていた場合は勝手に除外せず親へ返す。
- M5で `TestCommandBinary` とStandalone Japanese stepが削除され、stdin/空PATH保証が同候補 `TestReleaseCandidateNative` へ移る。Flow/GitIndependent Journeyは二つの代表へ、probe/liveは明示diagnostic tagへ整理される。M6で先取り実装しない。diagnostic/stressの明示手順を保持し、新たな自動CI stepへ増やさない。
- writer所有は `.github/workflows/ci.yml`、`.github/workflows/distribution.yml`、`docs/development.md` の検証手順、M6結果と親が割り当てた計画/RAM記録。製品Go、go.mod/go.sum、test file、Action version/SHA、cache方針、runner、timeout、権限、repository保護設定を変更しない。原 `/Users/const/sori883/ai-dd` に触れず、他者の編集を戻さない。
- `implementation-planning` / `golang-how-to` / `golang-continuous-integration` を適用。一般skillの新lint/securityサービス等は今回の範囲外。外部module/tool、汎用script framework、YAML snapshot testを追加しない。

## W01–W06の処置と実行責任

| ID | 処置 | 残る検査・所有者 |
|---|---|---|
| W01 | `ci.yml` の `cross-build` job全体を削除 | Distribution/Package、Go1.26.4、5CLI×6target、CGO=0/trimpath、同commitの7asset。1.26.x/stableのnative package compile/testはQuality。全Go版×全6targetの組合せ保証とはしない |
| W02 | matrixを維持し、format/vet/race/module/journey/dev smokeを1.26.xだけへ | 各Go版の通常testとworkspace/okf filesystem test、主要版の全package raceと明示的full vet |
| W03 | `aidlc-injected` buildと仮version/commit検査を削除 | 未注入aidlcのdev/unknown・help・未知引数stdout/stderr/exitを主要版の1buildで維持。実注入版はDistribution Nativeの5CLI完全一致検査 |
| W04 | 両workflowのpushをmainだけへ、PRは維持 | PR merge refとmain反映後の検査、Distribution手動実行。PR未作成branchのpushでは自動検査しない承認済み運用。push HEADとPR merge SHAを同じとは説明しない |
| W05 | exact workspace/okf importだけ通常一覧から除き、同Go版のintegrationへ集約 | 二つのpackageの通常test＋追加filesystem testを各版1回。raceは別の計測目的なので主要版 `./...` を維持 |
| W06 | 理由付き保持。入口存在、同run artifact ID、source/remote tag、draft直前照合を削らない | 6target Package、3OS Native、Windows PS5.1/7両engine、公開候補取り違え拒否。tag・draft・公開操作は実行しない |

## workflowの具体変更

両workflowのtriggerを次の形にする。Distributionは既存 `workflow_dispatch` 以下をそのまま残す。pull_request_target、path filter、追加concurrency/cancel条件は導入しない。

```yaml
on:
  push:
    branches: [main]
  pull_request:
```

Qualityの既存job名・matrix・fail-fast:falseを維持。Go値は文字列 `['1.26.x', 'stable']`。format、Vet、race、journey、module、native dev smokeの各stepへ次を付け、test/package選択stepには付けない。失敗時の既定停止を変える `always()` 等は使わない。

```yaml
if: ${{ matrix.go-version == '1.26.x' }}
```

通常testと二つのfilesystem stepを、一つの `shell: bash` stepで次の2回のgo testにする。Go listの失敗を隠すprocess substitutionは使わず、成功を確認して配列へ格納する。部分文字列による除外や全integration package一括実行はしない。

```bash
set -euo pipefail
workspace_pkg='github.com/sori883/ai-dd/src/internal/workspace'
okf_pkg='github.com/sori883/ai-dd/src/internal/okf'
all_packages="$(go list ./...)"
[[ -n "$all_packages" ]] || { echo 'package一覧が空です'; exit 1; }
normal_packages=()
workspace_count=0
okf_count=0
while IFS= read -r package; do
  case "$package" in
    "$workspace_pkg") workspace_count=$((workspace_count + 1)) ;;
    "$okf_pkg") okf_count=$((okf_count + 1)) ;;
    '') echo '空のpackage名です'; exit 1 ;;
    *) normal_packages+=("$package") ;;
  esac
done <<< "$all_packages"
[[ "$workspace_count" -eq 1 && "$okf_count" -eq 1 && ${#normal_packages[@]} -gt 0 ]] || {
  echo '通常packageとfilesystem対象の分割が成立しません'; exit 1;
}
printf 'normal packages: %s; filesystem packages: 2\n' "${#normal_packages[@]}"
go test -count=1 -shuffle=on -coverprofile=coverage-normal.out "${normal_packages[@]}"
go test -tags=integration -count=1 -shuffle=on -coverprofile=coverage-filesystem.out \
  "$workspace_pkg" "$okf_pkg"
```

二つのcoverage fileは別集合を表す。現在upload/閾値/集計の消費者はないためmerge処理を作らず、通常fileだけを「全package coverage」と呼ばない。matrixごとは別runner。`okfmemory` / `okfapp` / `cmd/okf` は通常一覧に残る。

主要版だけの既存commandは次を保つ。

```sh
go vet ./...
go test -count=1 -race -shuffle=on ./...
go mod tidy -diff
go test -tags=integration -count=1 ./src/cmd/aidlc -run '^Test(FlowJourney|GitIndependentJourney)$'
```

gofmtは既存の `unformatted="$(gofmt -l src)"`＋非空ならexit1を維持し、適用しない。明示的full vetを主要版1回へ集約する。go testの標準の限定vetは既存動作として残し、`-vet=off` を追加しない。raceは通常testの再実行でも別instrumentationの保証なのでW05の削除対象ではない。

journeyの旧「four-stage」step名はM5後の意味へ直す。実入口名が変わった場合は親が固定した生存名へregexを合わせる。M5のOps/OKF保存・diagnostic等を新たな恒常CI stepへ増設する作業ではない。

Dev smokeは現stepを縮小する。`go build -trimpath -o "$RUNNER_TEMP/aidlc" ./src/cmd/aidlc` の1回を残し、既存shellの成功/失敗取得・空出力確認で以下を検査する。専用runner/scriptを追加しない。

- `--help`: stdoutにUsage、stderr空。
- `--version`: stdoutが `aidlc dev (commit unknown)`、stderr空。
- `unknown`: exit2、stdout空、stderrに `aidlc: unknown arguments: "unknown"` とUsage。

## Distributionで維持する境界

`.github/workflows/distribution.yml` はtrigger以外を原則変更しない。prepareがcheckout SHAを確定しpackage/nativeが同じSHAをcheckoutする現在の仕組みを維持。

- prepare: manualはmain起動、既存tag形式・解決・main到達可能性を確認。通常はdev-SHA候補。
- package: Go1.26.4、5CLI×6target、license入力、同commit確認、Metadata/MetadataValidation、7asset。`-list`＋exact grepによる入口確認を維持。
- transfer/native: upload overwrite:false、同run artifact-idでdownload、digest-mismatch:error、3OS Native/bootstrap。候補を再buildしない。
- Windows: `TestBootstrapPowerShell` と `TestBootstrapCandidateNative` の `powershell.exe` / `pwsh.exe` 両方を維持。engine不在をskipへ変えない。3OS runnerは全6CPUの実起動保証ではない。
- draft: `needs: [prepare, package, native]`、manual/create_draft/成功条件、draft jobだけcontents:write、tag別concurrency/cancel:false、Metadata再検査を維持。remote tag再fetch/commit・main照合、Release全page取得の失敗時停止/同tag拒否、7files、`gh release create --verify-tag --draft`を維持。自動publish、clobber、自動削除を追加しない。

PRでdraftが条件付きskipなのは正常だが、package/nativeのskip・pending・未開始は成功ではない。公開境界を検証するために実tagやReleaseを作らない。

## 順序付きwork unitとtargeted事前確認

以下はwriter/親がM5後に実行する予定で、計画担当は未実行。CI設定の統合は既存正挙動の `ALREADY_GREEN` と扱い、人工REDやYAML文字列比較testを作らない。

1. **基準固定**: M5 merge後のbase、最終入口名、Go matrix、stdin移動とStandalone Japanese step削除を親が確認。生存journey/Distribution入口を静的に確認。
2. **W02/W05**: package選択shellを一時 `/tmp/ai-dd-m6-package-check.sh` へ切り出し `bash -n`。go test行を含まない選択部分だけを実行し、通常集合＋exact2packageがGo list全体を過不足なく覆うこと、各exact対象1回、normal非空を確認。空/失敗を成功扱いするpipeへ変更しない。
3. **tag前提**: 下記限定診断でproduction file集合が同じで追加testだけ増えることを確認。通常2packageを別に再実行する比較は不要。

   `go list -f '{{.ImportPath}} {{.GoFiles}} {{.CgoFiles}} {{.SFiles}}' ./src/internal/workspace ./src/internal/okf`

   `go list -tags=integration -f '{{.ImportPath}} {{.GoFiles}} {{.CgoFiles}} {{.SFiles}}' ./src/internal/workspace ./src/internal/okf`

   `go list -tags=integration -f '{{.ImportPath}} {{.TestGoFiles}} {{.XTestGoFiles}}' ./src/internal/workspace ./src/internal/okf`

4. **W01/W03/W04**: cross-buildと注入smokeを削除、主要版if/main filterを追加。変更shellを一時fileへ切り出し `bash -n`。smoke実行はfinalへ。YAML parser/toolを追加せず、matrix式・trigger・step所属はreviewと実GitHub checksで確認。
5. **W06/手順**: 保持表を差分で照合し、手順へ所有job/Go版/coverage2fileとdiagnostic/stress入口を記載。親が必要とするcompile/discoveryは以下だけ。名前をexact照合し、ゼロ件成功を認めない。

   `go test -tags=integration -list '^Test(FlowJourney|GitIndependentJourney)$' ./src/cmd/aidlc`

   `go test -tags=integration -list '^TestReleaseCandidate(Metadata|Native)$' ./src/cmd/aidlc-dist`

   `go test -tags=integration -list '^TestBootstrap(PowerShell|CandidateNative)$' ./src/bootstrap`

   `git diff --check`

親が末尾の差分・targetedを一度確認し、固定base/headの独立reviewへ渡す。reviewerはmatrix/集合/公開gateを確認し、全testやcrossbuildを代行しない。

## final・GitHub合否・完了条件

差分安定後に親だけがread-only finalを開始する。ローカルで利用可能な主要Go1.26.xの実版を記録し、package選択部分を実行した同じbash環境で以下を実行する。未確認版を主要版成功と記録せず、未導入版を自動導入しない。二つのGo版の正式確認はGitHub Qualityが所有する。

```bash
# normal_packages/workspace_pkg/okf_pkgは上のpackage選択部分で設定する。
go test -count=1 -shuffle=on -coverprofile=/tmp/ai-dd-m6-normal.out "${normal_packages[@]}"
go test -tags=integration -count=1 -shuffle=on -coverprofile=/tmp/ai-dd-m6-filesystem.out \
  "$workspace_pkg" "$okf_pkg"
go test -count=1 -race -shuffle=on ./...
go vet ./...
go mod tidy -diff
unformatted="$(gofmt -l src)"
[[ -z "$unformatted" ]] || { printf '%s\n' "$unformatted"; exit 1; }
git diff --check
go test -tags=integration -count=1 ./src/cmd/aidlc -run '^Test(FlowJourney|GitIndependentJourney)$'
```

Dev smokeはworkflowの同じshellを親finalで1回実行。ローカルでは `RUNNER_TEMP` 相当の一時出力先を与え、実Go版・対象head・結果を記録する。M3/M5のdiagnostic/stressは低頻度入口を維持し、M6で全面再実行や外部Codex liveを前提にしない。全30binaryのローカル再buildを行わない。

対象PRの現headに対応するGitHub実行で次を全て確認する。PR merge SHAは作業headと異なり得るため、候補SHAと当該PRの対応も照合する。

- `Quality (Go 1.26.x)` / `Quality (Go stable)` が成功。両方でnormal/filesystemが実行され、stableで主要版専用stepだけ意図通りskipする。resolved Go版も記録。
- 主要版race/vet/format/module/journey/dev smokeが実行され成功。削除したcross-build/Standalone Japaneseを未実行の成功gateとして数えない。
- Distribution/prepare、Package六target、Native ubuntu/macos/windowsが同候補で成功。Windows logでPS5.1/7、同候補bootstrap、M5 stdin検査が実行されたことを確認。
- PR checksが未開始/pending/failure/cancelならmergeしない。branch protection/rulesetを変更せず、親が承認済み合否gateを管理。

全checks成功後、親がIssueへ紐付く日本語PRを既存方式でmergeし、main反映・Issue close・main pushの新workflow起動を確認する。main側のfailureは完了扱いせず修復する。W01–W06の処置と残るjobを結果記録へ。final後の変更は証拠をstaleとし必要なloop/review/finalへ戻る。rollbackは当該PRをrevertする別PRで、他の区切りや公開物をresetしない。

新製品仕様や未承認選択はない。M5後のpackage/tag/入口が想定と違えば、保証を弱めず親が詳細を更新する。現在のGo版対応・3OS/2PS・公開前照合を減らす判断は本計画に含めない。
