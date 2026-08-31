# 共通space読み取りの初期契約

- 日付: 2026-08-31
- 状態: Accepted
- GitHub Issue: [#13](https://github.com/sori883/ai-dd/issues/13)
- 関連記録: [内部機能を先行する実装順序](2026-08-31-internal-workspace-before-status.md)、
  [参照契約の調査](../research/2026-08-31-space-reading-contracts.md)

## 承認と背景

ユーザーは、共通space読み取り2機能、標準ライブラリのみ、TDDと実filesystemでの検証、
独立レビュー、Issueに紐づくPRまでの計画を「はい、お願いしてもいいですかね。」と承認した。
続くspace作成機能の有無についての質問は現状確認として扱い、作成・切替を実装範囲に
追加する承認とは扱わない。自動マージはしない。

本家v2.6.123の既存データを読み取れる最小機能として、内部root解決の次に実装する。
この記録が、関連する順序・調査記録に残る「詳細計画は承認待ち」の後続承認に当たる。
先行記録の経緯は変更しない。

## 決定したAPIと境界

`src/internal/workspace`が、project root基準の`fs.FS`を注入して次のAPIを提供する。

```go
type Space struct {
    Name   string
    Active bool
}

func ActiveSpace(projectFS fs.FS) string
func ListSpaces(projectFS fs.FS, activeOverride *string) []Space
```

新しいstore層、独自FS interface、外部Go module/toolは追加しない。
cwdや環境変数を直接読まない。root.goと公開CLIの契約は変更しない。

## 受入契約

1. `ActiveSpace`は`aidlc/active-space`を読み、JavaScriptのtrim相当の処理を行う。
   空・未作成・その他のread errorでは`default`を返す。
2. cursorの形式・名前・存在を検証しない。返した名前を使う追加pathアクセスもしない。
3. `ListSpaces`は`aidlc/spaces`直下のdirectoryを列挙し、`default`を必ず1件含める。
   directoryへのsymlinkも供給FSのStatを通じて扱い、再帰はしない。
4. 最終一覧をJavaScriptのUTF-16コード単位順で整列する。`default`先頭固定ではない。
5. 選択中フラグは名前の完全一致とする。overrideがnilの場合だけcursorを読む。
   非nilの空文字も明示値とし、overrideをtrimしない。未知名では全件inactiveになり得る。
6. ReadDir失敗時は`default`だけを返す。子のStat失敗時は列挙を打ち切り、
   それまでの収集分と`default`を返す。
7. ファイル作成・書込・初期化・修復・cursor切替は行わない。

read errorを吸収する点は本家との互換性のため維持する。このAPIだけでは未作成と
権限不足等を区別できないことを、利用側が理解できるよう文書化する。

## 検証と変更範囲

- `space.go`、`space_test.go`、`space_integration_test.go`をworkspace packageへ追加する。
- fixtureとerror注入stubで、通常値、空・欠損・失敗、部分dataとerror、未検証名、
  override優先順位とcursor未読、途中Stat失敗、重複除去を失敗先行で検証する。
- JS/Goのtrim差（U+FEFF、U+0085）とsort差（非BMP名）をfixtureに含める。
- integration build tagで実filesystemとsymlink、broken link、製品処理による書込みが
  ないことを検証し、既存CIのquality jobへ実行工程を追加する。
- `docs/architecture.md`と`docs/development.md`へ責務・実行手順を追記する。
- 全テスト、race、vet、gofmt、module metadata、diff、既存6 target buildを確認する。
  cross-buildを各OSの実行証拠とは扱わない。

テストfixtureとして一時directoryやファイルを用意することと、Go製品にspace作成機能を
追加することは別である。公開CLIから未到達のため、配布E2Eは今回の検証範囲に含めない。

## 保証対象外・後続事項

- 不正UTF-8のdecode、およびStat失敗時のOS/runtime別列挙順による部分集合の完全互換。
- `os.DirFS`によるsandbox化。symlinkはroot外も参照し得るため、名前からpathへ到達する
  後続機能ではfilesystem境界と検証を別途設計する。
- spaceの作成・切替・削除、名前からのpath解決、Knowledge専用検証。
- 明示指定/sessionを含む最終workflow選択、intent、state、status、新規CLIコマンド。

## 作成機能の現状

本家v2.6.123には`space create <name>`と`space switch <name>`が存在し、作成だけでは
選択は切り替わらない。Go版にはどちらもまだ実装していない。今回の読み取り実装には
追加せず、後続の別計画で扱う。

根拠: ローカル参照の`core/tools/aidlc-utility.ts`の`handleSpaceCreate`（6994行付近）と
`docs/guide/03-spaces-and-intents.md`（234行付近）。
