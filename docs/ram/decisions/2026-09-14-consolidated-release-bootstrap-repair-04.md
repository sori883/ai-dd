# Coverage子プロセス診断のtest修復04

Issue #202 / PR #203、work_unit consolidated-release-bootstrap-repair-04、loop、開始HEAD 9c290d1。
CI34772300421でstderrにsentinelの後、Go coverageのprogram not built with -cover診断が追加され、
完全一致assertionが失敗した。stdout JSONは正しい。親の承認に従いtestだけを最小修復する。
stdoutの完全一致、stderrの先頭sentinelとJSON非混在、終了値0/17が本来の観測契約。
追加ホスト診断の存在はstream分離違反ではないため許容する。CombinedOutputなら引き続き失敗する。
全package coverageは親finalに任せ、loopではbootstrap packageだけを確認する。


RED/GREEN:
`go test -count=1 -coverprofile=/tmp/ai-dd-repair04-cover.out ./src/bootstrap -run '^TestBootstrapOutputStreams$'`
で0/17両caseの追加coverage診断によるassertion RED exit 1を再現。
stderrを先頭sentinel＋JSON非混在のassertionへ変更し、同command GREEN exit 0。

末尾は`go test -count=1 ./src/bootstrap`と
`go test -count=1 -coverprofile=/tmp/ai-dd-repair04-cover.out ./src/bootstrap`がともにexit 0。
testだけのpackageなのでcoverage表示は[no statements]だが、対象testは実行済み。
gofmt適用、git diff --check成功。PowerShell動的実行と全package coverageは親CI/finalへ残す。
