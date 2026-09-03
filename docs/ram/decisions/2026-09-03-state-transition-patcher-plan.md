# byte-preserving state transition patcherの実装計画

- 日付: 2026-09-03
- 状態: Accepted
- GitHub Issue: [#73](https://github.com/sori883/ai-dd/issues/73)
- 実装許可: [薄いライフサイクルマイルストーン](2026-09-03-thin-lifecycle-milestone.md)とロードマップ包括承認内

## 現状と目的

現在の`state.Parse`は、保存済み`aidlc-state.md`を厳密に検証してtyped snapshotを返すが、入力byteを保持しない。このsnapshotから文書全体を再生成すると、将来追加されたfield、利用者のコメント、空白、改行形式を失う。

本変更では、検証済みのraw Markdownに対して、ライフサイクル遷移が所有するcanonical field、phase status、Stage markerだけを指定して置換する純粋なpatcherを追加する。変更対象以外のbyteを保持し、後続のwriterやorchestratorが安全な新state内容を構築できるようにする。

## 設計

- 入力raw bytesを最初に`state.Parse`で検証し、malformed stateには変更を適用しない。
- 変更対象は型付きのfield、phase、Stage markerで指定し、任意のMarkdown文字列や未知sectionを変更するAPIにはしない。
- 各対象はcanonical section内で一意に存在する場合だけ置換し、missing、duplicate、marker前提不一致、同一対象の重複指定を拒否する。
- Stage markerはslug、期待する現在marker、置換後markerを受け、suffixをbyte単位で維持する。marker遷移の業務上の許可判断は後続orchestratorが所有する。
- field値は改行やMarkdown構造を注入できないscalarだけを許可し、対象fieldの既存enum・数値・slug契約に従う。
- BOM、LF/CRLF、終端改行の有無、未知section、コメント、未変更field、空白を保持する。
- patch後の文書を再度`state.Parse`し、整合しない結果は返さない。
- filesystem I/O、state保存、audit、lock、clock、完了可否や承認判断は行わない。

## 対象ファイル

- 新規`src/internal/state/patch.go`
- 新規`src/internal/state/patch_test.go`
- 必要最小限の`src/internal/state/state.go`または既存parser helper
- `docs/architecture.md`
- `docs/development.md`
- `docs/ram/README.md`
- 本計画

実装担当は上記を単独所有し、他の作業者の変更をrevertしない。

## TDD slice

1. `[-]`から`[?]`、`[?]`から`[x]`、`[?]`から`[R]`、`[R]`から`[?]`のmarker置換でsuffixと周辺byteを保持する。
2. Current/Next、Summary、Lifecycle Phase、Status、Phase Progressなど、後続advanceが必要とするcanonical fieldを型付き指定で置換する。
3. unknown section、コメント、空白、BOM、CRLF、終端改行の保持をexact bytesで確認する。
4. missing、duplicate、decoy、malformed、expected marker不一致、同一対象の重複指定を拒否し、入力sliceを変更しない。
5. patch後を`state.Parse`し、zero patchと不正な結果をfail-closedにする。

loopではstate packageのpatcher対象testだけを実行する。full、race、vet、cross buildは独立review後のfinalへ集約する。

## 受け入れ条件

- 変更対象以外のbyte範囲を保持する。
- BOM、LF/CRLF、終端改行の形式を保持する。
- suffix-only routing authorityを変更しない。
- malformedまたは曖昧な対象を部分変更せず拒否する。
- patch後の文書が既存`state.Parse`を通過する。
- filesystem I/Oや外部Go moduleを追加しない。

## 互換性・リスク

raw文書の局所置換を採用することで、canonical full rendererによる未知内容の消失を避ける。固定AI-DLC 2.6.123で確認したmarkerとstate fieldを対象とし、新しい意図的な仕様・挙動差分は採用しない。

patcher単体は遷移の正当性、同時更新、永続化atomicityを保証しない。これらは後続のgate、record lock、audit、atomic writerが所有する。

## APIと実装記録

公開する内部APIは次のとおりである。

```go
func Patch(content []byte, request PatchRequest) ([]byte, error)
```

`PatchRequest`は、`CanonicalField`を持つ`FieldPatch`、`LifecyclePhase`と`PhaseStatus`を持つ
`PhaseProgressPatch`、slugと`StageMarker`を持つ`StageMarkerPatch`から構成する。fieldのallowlistは
`Total Stages`、`Completed`、`In Progress`、`Lifecycle Phase`、`Current Stage`、`Next Stage`、`Status`、
`Last Updated`に限定し、requestにない任意Markdown targetは受け付けない。Patchはerror時にnil bytesだけを返し、
input slice、filesystem、state、audit、lock、clockを変更しない。

`loop`実装記録（2026-09-03）:

- marker sliceの最初のREDは`Patch`／request／marker型未定義のcompile failure。最小GREENで`[-]`、`[?]`、`[R]`を含む4遷移、suffix、BOM、CRLF、unknown section、input ownershipを固定した。
- field sliceは未実装拒否をREDとして観測し、8件のcanonical fieldをtyped value validation付きで置換した。unknown／missing／duplicate／decoy／expected mismatchとscalar injectionを拒否する。
- phase sliceは未実装拒否をREDとして観測し、canonical phase statusを局所置換した。patch前後の`Parse`、zero／malformed／ambiguous inputのnil partial resultを確認した。
- 代表的なtargeted GREENは`go test -count=1 -run '^TestPatch' ./src/internal/state`。変更Go fileはgofmt済みで、独立review前に`gofmt -l`と`git diff --check`を再確認する。

実装時点の残余riskは、marker遷移の業務上の許可、stateのatomic persistence、audit、record lock、上位callerによる
同時更新制御をこのpure patcherが扱わないことである。
