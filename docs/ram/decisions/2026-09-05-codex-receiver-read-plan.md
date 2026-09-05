# Codex receiverで配信本文を実読込する

- 日付: 2026-09-05（Asia/Tokyo）
- 状態: Implemented（live実読込receipt成功、独立review・final検証待ち。Issue #115）
- 対応Issue: [#115 Codex receiverで配信本文を実読込する](https://github.com/sori883/ai-dd/issues/115)
- 実装許可: [ルール・知識のAI供給を個別承認なしで完了まで進める](2026-09-05-context-delivery-autonomous-authorization.md)
- 前提: [Codex向け配信transactionと公開next／continueを接続する](2026-09-05-delivery-publication-plan.md)
- 基準: リポジトリ固定AI-DLC 2.6.123、OpenAI Docsのrepo skill discovery契約

## 背景と利用者が得る結果

PR #114で、Go単一binaryの`aidlc next`／`aidlc continue <token>`が、必須rule本文を
`load-steering`として順番に公開し、全chunkの一回限り受領後だけcanonical `run-stage`を返すようになった。
しかしCodexがこの公開入口を呼び、pathの一覧を得るだけでなく、選択済みpersona／knowledge、工程手順、
入力成果物の本文を実際に読むreceiverはまだない。

本Issueでは、Codexがrepoから発見できる`aidlc` skillの配布用sourceを追加する。fresh projectへ
Go binaryとskillを明示配置したとき、Codexは全rule chunkを蓄積してから、`run-stage`が示す
`inline_context_paths`を全て先に読み切り、その後に`stage_file`と存在する全`consumes`を本文まで読む。
これにより、ファイル名の列挙ではなく、予測不能なrule本文の最後の非空行sentinelと、inline／stage／consume本文をCodexの
最終応答で再現できるところまでを「Codexへの配信・実読込」の完了条件とする。

## 実装許可と作業順から決まる境界

知識供給の包括承認は、Codex側の受領・実読込、Issue、PR、品質gate後の自律mergeまでを直接許可している。
ユーザーが一般installer／updateを全体作業の第5項目に置いたため、今回は既存環境を上書きするinstallerを作らず、
fresh projectへbinaryとreceiverを明示配置するE2Eを採用する。この選択は公開上書き・移行規則を新設せず、
包括承認と作業順から一意に決まる。

固定本家のCodex skillは、Stage実行、review、sensor、修正、report、人間承認まで含む。一方、Go版で現在利用可能なのは
配信とcontext構成までで、次の作業項目が「本番Stageを1つだけ縦に通す」である。本receiverはcontextを読み切ったら
Stage完了を装わず停止する。これは段階的実装の境界であり、固定本家から恒久的に挙動を変える判断ではない。

## 確認済み契約

- 固定本家2.6.123の`harness/codex/manifest.ts`は、orchestrator skillを
  `.agents/skills/aidlc/SKILL.md`へ配置する。
- 配置済み固定本家skillの`load-steering`分岐は、`rules_content`を配列順で保持し、進捗を報告せず、
  `continue_token`を直ちに継続し、`report`しない。
- 同skillの`run-stage`分岐は、全`inline_context_paths`の読込をblocking preconditionとし、全ての結果を待ってから
  `stage_file`と`consumes`を読む。pathの存在確認や一覧取得は本文読込の代わりにならない。
- OpenAI Docsは、Codexがrepository rootの`.agents/skills`を走査し、`SKILL.md`の`name`と`description`で
  明示・暗黙にskillを選び、選択後に本文全体を読むことを定める。
- `codex exec`は非対話実行で`--output-last-message`へ最終agent messageを書けるため、fresh projectでの実読込証拠を専用receipt fileから自動検査できる。stdout/stderrは失敗診断に限る。

## 設計

### 配布用receiver source

正本は`src/harness/codex/skills/aidlc/SKILL.md`とし、利用時の配置先は
`.agents/skills/aidlc/SKILL.md`とする。開発repository自身の`.agents/skills/`へ同名skillを追加すると、
本repositoryの開発支援skillと製品配布物が混ざるため行わない。一般配布時のcopy／update機構は第5項目へ残す。

receiverはPATH上のGo binaryを、project rootから次のように呼ぶ。

- 開始: `aidlc next --project-dir .`
- 継続: `aidlc continue "<opaque token>" --project-dir .`

fresh E2Eではbuild済みbinaryを一時projectの専用`bin`へ置き、そのdirectoryだけをsubprocessのPATH先頭へ加える。
既存binaryやuser設定を変更しない。tokenを解析・再構成せず、一つの引数として引用して渡す。

### directive処理

1. `load-steering`は`rules_content`をpart順に全て保持し、user向け進捗や`report`を挟まず直ちに継続する。
2. `error`はmessageを示して停止し、fresh `next`等をreceiver判断で自動再試行しない。
3. `run-stage`は、全`inline_context_paths`を最初のfile read群として本文まで読み、全結果を待つ。
4. inline読込完了後だけ`stage_file`を本文まで読み、続いて`consumes`を配列順で全て本文まで読む。
5. いずれかの必須readが失敗したら停止し、Stage実行・artifact生成・reportを行わない。通常呼出しの全read成功時は
   `context ready`だけを報告し、Stage完了や人間承認を主張しない。callerが検証用machine-readable read receiptの要求と
   output schemaを明示した場合だけ、そのschemaが要求するreceiptだけを返して停止する。この例外もStage実行・artifact生成・
   report・review・人間承認へ進む許可にはならない。
6. 未対応directive kindを推測で実行せず、安全に停止する。

`context_warnings`は読込前に利用者へ示すが、warningだけを理由にpathを省略しない。保持したrule本文は、
将来のStage実行へ渡せるactive bundleとして扱い、本Issueではartifactへ書き出さない。

### 検証面

通常のGo testは、skill frontmatter、公開command、directive分岐、読込順、Stage未実行境界が失われていないことを
構造的に検査する。skill作成用validatorも実行し、未完成placeholderや不正frontmatterを拒否する。

integration testはrepository外のfresh projectを作り、source skillを正規配置先へbyte一致でcopyし、build済み
`aidlc`を一時PATHへ置く。複数chunkになる2つのrule、lead／supportのinline context、stage file、consumeへ
実行ごとに`crypto/rand`で作る別々のsentinelを置き、公開CLIがそれらのpathだけを順序付きで返すことを検査する。

live test `TestCodexReceiverReadsDeliveredContext`は`AIDLC_CODEX_EXEC_LIVE=1`のときだけ、既存の
`codex exec --ephemeral`を1回起動する。live promptは検証用machine-readable read receiptの要求とoutput schemaを明示するため、
通常の`context ready`ではなくschemaに従うreceiptだけを返させる。promptにはexpected sentinel値やfile pathを渡さず、repo skillとdirectiveに
従って得た本文だけから、`rules`には各ruleの最後の非空行を順序どおり、`inline_context`／`stage_file`／`consumes`には全本文を含むJSON
receiptを返させ、`--output-last-message`で専用temporary receipt fileへ保存する。rule sentinelと全context本文の順序・完全一致で、Codexが
本文を実際に読んだ証拠とする。環境変数がない通常CIでは明示skipし、credentialを読取・複製しない。

`codex exec`は外部model利用量を消費し得るため、包括承認から認証情報・有料serviceの許可を推測しない。
実装と非live検証を完了した後、live commandを実行する直前にユーザーへ1回だけ明示確認する。

## 対象fileと単独writer所有権

1人のGo実装担当が、次を1 work unitとして所有する。

- `src/harness/codex/skills/aidlc/SKILL.md`: 配布用receiver source。
- `src/harness/codex/skills/aidlc/skill_test.go`: 通常のskill契約test。
- `src/cmd/aidlc/codex_receiver_integration_test.go`: fresh配置journeyとlive-gated Codex実読込。
- `docs/architecture.md`, `docs/development.md`, `docs/e2e-testing.md`: 配布、停止境界、再現command。
- 本記録と`docs/ram/README.md`: 実装・live証拠の記録。

実装担当はIssue・PR・live `codex exec`を操作せず、同じworktreeの他者の変更を戻さない。

## 1つのwork unitで行うTDD

`work_unit_id=codex-receiver-context-read`として、次を順番にtest-firstで実装する。

1. `receiver-skill-contract`
   - frontmatter、`aidlc next`／`continue`、全rule保持、error停止、inline全件→stage→consume全件、
     Stage未実行停止の構造契約を失敗testで固定する。
   - `go test -count=1 ./src/harness/codex/skills/aidlc`
2. `fresh-placement-journey`
   - repository外projectへsource skillとbinaryを明示配置し、複数rule chunkから`run-stage`へ進め、
     directiveの全context pathを順序付きsentinelへ解決できることを確認する。
   - `go test -tags=integration -count=1 -run '^TestCodexReceiverFreshPlacementJourney$' ./src/cmd/aidlc`
3. `live-gated-read-receipt`
   - 環境変数なしでは明示skipし、環境変数ありでは`codex exec` 1回からrule sentinelと全context本文の順序一致receiptを得る。
   - 非live loop: `go test -tags=integration -count=1 -run '^TestCodexReceiverReadsDeliveredContext$' ./src/cmd/aidlc`
   - live gate承認後: `AIDLC_CODEX_EXEC_LIVE=1 go test -tags=integration -count=1 -run '^TestCodexReceiverReadsDeliveredContext$' ./src/cmd/aidlc`

work unit末尾では上記non-live test、影響package test、変更Go fileの`gofmt`、skill validator、
`git diff --check`を実行する。loop中に全package、race、vet、cross compileを繰り返さない。

## 実装記録（work unit loop）

`work_unit_id=codex-receiver-context-read`の3 sliceを、単独writerで順番に実装した。`receiver-skill-contract`では未配置
`SKILL.md`を読むtestがREDとなり、frontmatter、PATH command、rule順序、opaque token、run-stageのread順、fail-closed、
context-only境界を定めたsource追加後にGREENとなった。`fresh-placement-journey`ではrepository外fresh projectへsourceを
byte-identical配置し、`crypto/rand` sentinel付きの複数rule chunkをcontinueして、inline persona全件→stage file→consumeの
順で本文を照合するtestがGREENとなった。rule本文は全文をfresh journeyで照合し、live receiptでは各ruleの最後の非空行sentinelだけを返す。

`live-gated-read-receipt`では、外部model利用とcredential境界を越えないため、`AIDLC_CODEX_EXEC_LIVE`が`1`でない場合に
明示skipするtestだけを追加した。通常呼出しは全read後に`context ready`だけを返し、live promptのようにcallerが検証用receiptと
schemaを明示した場合だけschemaに従うreceiptを返す限定例外をSKILLへ記録した。work unitのnon-live loopでは環境変数を設定せず、
testがskipすることだけを確認したため、その時点ではlive receipt成功を主張しなかった。

## live実読込証拠

ユーザーは2026-09-05、現在ログイン済みのCodexアカウントで`codex exec --ephemeral`を1回だけ実行し、利用量を消費することを
明示的に許可した。この許可後、source commit `11c11e4341e4d6fb825e79c24d8d9ee3266dd3f7`に対して次を1回実行した。

```sh
AIDLC_CODEX_EXEC_LIVE=1 go test -tags=integration -count=1 -run '^TestCodexReceiverReadsDeliveredContext$' -v ./src/cmd/aidlc
```

`go1.26.4 darwin/arm64`、`codex-cli 0.145.0`で、testは107.85秒、packageは108.316秒でexit 0となった。fresh projectで
repo skillを発見したCodexが、promptに値もpathも与えられない状態から、2つのrule末尾sentinelを配信順に、lead／supportの
inline context全文を先に、続いてstage file全文、最後にconsume全文をJSON receiptへ返し、testが全byteと順序の完全一致を確認した。
実行は`--ephemeral`、`--skip-git-repo-check`、`--sandbox workspace-write`、`approval_policy="never"`を使い、専用temporary
`--output-last-message`だけをreceiptとして検査した。追加のlive retryは行っておらず、Stage実行・成果物生成・report・人間承認も
行っていない。

## 独立reviewとfinal検証

live receiptと実装記録を含む差分が安定した後、独立reviewで、固定本家のread順、rule蓄積、opaque token、
未対応kindのfail-closed、PATH／project root境界、live testの予測不能性、credential非取扱い、
Stage／report／approvalを越境しないことを確認する。

blocking finding解消後、親がread-only finalを1回実行する。

- `go test -count=1 -shuffle=on ./...`
- `go test -tags=integration -count=1 -shuffle=on ./...`（live envなし、skip契約を確認）
- 通常／integrationの`go test -race`と`go vet`
- `go mod tidy -diff`、`gofmt -l src`、変更Go fileへの`gopls check`、`git diff --check`
- skill validator
- darwin／linux／windows × amd64／arm64のCLIと対象test binary cross compile
- fresh配置journey

final後に対象fileが変わった場合は証拠をstaleとし、targeted loopと再review後にfresh finalへ戻す。

## 受け入れ条件

- 配布用sourceが公式・固定本家と同じrepo discovery先へ配置でき、一般installerなしのfresh projectで発見される。
- Codexが全`load-steering`本文をpart順に保持し、最終partまで`run-stage`処理を始めない。
- Codexが全inline contextを先に、次にstage file、最後に全existing consumeを本文まで読む。
- file名だけでなく、promptから予測不能なruleの最後の非空行sentinelを順序どおり、inline／stage／consume本文を完全一致でlive receiptが返す。
- 読込・directive failureではStage実行、artifact生成、report、approvalへ進まず、成功時もStage完了を主張しない。
- user環境、credential、既存skillを変更せず、外部Go moduleと新しい意図的な本家差分を追加しない。
- 独立review、fresh final、現在headのGitHub checks後にmergeし、Issueをcloseする。

## リスクとrollback

最大のリスクは、skillがpath一覧を読込済みと誤認すること、rule途中でStage処理へ進むこと、未対応Stage実行を
有効化することである。rule sentinelとcontext本文のlive receipt、blockingな二段階read順、未知kind／read failure停止で抑える。
問題時はreceiver sourceと専用test／docsをrevertでき、PR #114のGo配信facadeや永続cursor migrationは不要である。
