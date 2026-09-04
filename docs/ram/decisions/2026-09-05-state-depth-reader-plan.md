# 状態Markdownから工程のDepth設定を厳密に読み取る

- 日付: 2026-09-05（Asia/Tokyo）
- 状態: Accepted（知識供給の包括承認内）
- GitHub Issue: [#101](https://github.com/sori883/ai-dd/issues/101)
- 実装許可: [ルール・知識のAI供給を個別承認なしで完了まで進める](2026-09-05-context-delivery-autonomous-authorization.md)
- verification mode: `loop`（独立review後の`final`は親エージェントが1回だけ実施）

## 背景と目的

AI-DLCは、工程と担当AIに渡す知識を選ぶとき、保存済み`aidlc-state.md`の`Depth`を参照する。
固定AI-DLC 2.6.123では`Minimal`のときだけ、工程と担当AIの対応表に基づいて同梱知識を絞る。
Go実装は初期stateへDepthを保存できるが、保存済み本文から値を取り出す内部APIがない。

このIssueでは、後続の配信構成処理が開始時に確定したDepthを安全に取得できるようにする。
state本文全体の妥当性を先に確認し、正規の`Scope Configuration` sectionにある一意な`Depth`だけを返す。

## 固定した根拠

比較対象はリポジトリに固定したAI-DLC 2.6.123の次の範囲であり、最新upstreamとの一致は主張しない。

- `core/tools/aidlc-utility.ts:5922-5927`: 初期stateの`Scope Configuration`へ`Depth`を保存する。
- `core/tools/aidlc-orchestrate.ts:3184-3189`: run-stage組立て時にstateの`Depth`を読む。
- 同`:2943-3060`: `Minimal`の場合だけ工程・担当AI別の同梱知識を絞る。
- `core/tools/aidlc-lib.ts:16479-16488`: 本家の`getField`は値をtrimして返し、欠落時は`null`を返す。

Goの既存`state.Parse`と`canonicalSectionLines`は、曖昧なstateを誤ったroutingへ使わないため、対応section
内のfieldを一意に要求する。本家が文書全体の最初の同名fieldを使う点との違いは、Issue #63で承認済みの
fail-closed差分である。この計画はその境界を再利用し、新しい意図的な仕様差分を追加しない。

## 実装範囲とAPI

Go実装担当の所有対象は次の2ファイルに限定する。

- `src/internal/state/document.go`
- `src/internal/state/document_test.go`

追加する内部APIは次のとおり。

```go
func Depth(content []byte) (string, error)
```

`Depth`は次の順で処理する。

1. 既存`Parse`へ入力bytesを渡し、State Version 8の既存構造契約を満たすことを確認する。
2. `canonicalSectionLines(content, "Scope Configuration")`でexactなsectionを1つ要求する。
3. `requiredStringField(lines, "Depth")`でexactなfieldを1つ要求する。
4. 既存field helperがtrimしたnonempty値を返す。

section外の`Depth`は無視する。sectionまたはfieldの欠落・重複・空値、Parse不成功はerror chainに
`fs.ErrInvalid`を保持して失敗し、空文字以外のpartial結果を返さない。入力bytesは変更しない。

## 対象外と維持する境界

- `Minimal`、`Standard`、`Comprehensive`への値制限やcanonicalizeは追加しない。本家の知識選択は
  `minimal`だけを特別扱いし、それ以外では全候補を維持するためである。
- scope metadata、初期入力、graphへのfallbackは行わない。実行中のauthorityは保存済みstateである。
- filesystem I/O、stateの更新、知識roster、配信chunk、token、CLI、工程実行は変更しない。
- typed`State`へfieldを追加せず、state schemaやwriterの保存形式を変えない。
- 標準ライブラリだけを使い、外部Go moduleを追加しない。
- `orchestrator.Next`のread-onlyと、未対応工程能力のfail-closedを維持する。

現在の本家stage catalogとGoの実行能力の境界は、
[知識配信を工程へ接続する前提調査](../research/2026-09-05-context-delivery-stage-prerequisites.md)へ記録する。
このIssueはその能力を省略・実装せず、Depth取得だけを完成させる。

## TDD単位と受け入れ条件

1. `TestDepthReadsUniqueCanonicalScopeConfigurationField`を先に追加し、API未実装のREDを確認する。
   canonical値、前後空白のtrim、section外decoyの無視、入力bytes不変を固定する。
2. 同じtestを変更せず、`Parse`、`canonicalSectionLines`、`requiredStringField`を組み合わせる最小GREENを追加する。
3. `TestDepthRejectsInvalidStateOrAmbiguousField`を先に追加する。section／fieldの欠落・重複・空値、
   既存Parse不成功の全caseが`fs.ErrInvalid`になることを固定する。
4. 既存GREENであれば`ALREADY_GREEN`として記録し、人工的な不具合は入れない。不足があればtest不変の最小GREENを追加する。

各RED/GREENは別依頼とし、親がexact test commandを再実行してtest hash不変を確認する。compile failureは
behavior REDとして受理しないため、最初のREDでは同じtest file内に一時的なcompile可能stubを置く。

loop commandは次に限定する。

```sh
GOTOOLCHAIN=go1.26.8 go test -count=1 -run '^TestDepthReadsUniqueCanonicalScopeConfigurationField$' ./src/internal/state
GOTOOLCHAIN=go1.26.8 go test -count=1 -run '^TestDepthRejectsInvalidStateOrAmbiguousField$' ./src/internal/state
```

## reviewとfinal gate

実装完了後、固定base/headで独立reviewを行い、Issue範囲、既存承認済み差分、error分類、観測可能な回帰testを確認する。
blocking findingを解消し差分が安定した後、親がread-onlyの`final`を1回だけ実行する。

- 全package test、race、integration tag付きtest/race。
- 通常・integrationの`go vet`、`go mod tidy -diff`、`gofmt -l src`、対象fileの`gopls check`。
- `git diff --check`。
- CLIとstate integration test binaryをdarwin/linux/windows × amd64/arm64の6構成へcross compileする。

cross compileは各OS上の実行証拠とはしない。対象変更後はfinal証拠をstaleとしてloopへ戻し、再review後にfinalをやり直す。

