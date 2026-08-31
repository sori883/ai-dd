# 共通space読み取りの参照契約

- 日付: 2026-08-31
- 状態: Current for local v2.6.123 snapshot
- 対象: 共通libの`activeSpace`と`listSpaces`
- 方法: ローカル実装、canonical Codex配布、配置済み配布と関連テストの静的調査
- 実装状態: Goへの移植は未着手。以下の観測事実と実装候補の承認を区別する。

## 参照範囲

対象の`aidlc-lib.ts`は、次の3箇所で同一だった。

- `docs/実装_aidlc-workflows/core/tools/aidlc-lib.ts`
- `docs/実装_aidlc-workflows/dist/codex/.codex/tools/aidlc-lib.ts`
- `docs/配布_ai-dlc/.codex/tools/aidlc-lib.ts`

参照実装や配布物のinstaller、CLI、テストはこの調査では実行していない。

## activeSpace

- `<projectDir>/aidlc/active-space`をUTF-8テキストとして読み、全体にJavaScriptの
  `trim()`を適用する。空でなければその文字列を返す。
- ファイル不存在、空、空白のみ、その他の読み取りエラーでは`default`を返す。
- space名の形式やディレクトリの存在は検証しない。未知名、複数行、`../outside`なども
  外周の空白を除いた値として返り得る。
- spaceが1つだけ存在しても、その名前へ自動的に切り替える処理はない。
- cursorの作成、修復、切替は行わない。

## listSpaces

- 名前と選択中フラグを持つ`{ name, active }`の一覧を返す。名前だけの一覧ではない。
- `activeOverride`が指定されればcursorを読まない。空文字も指定値として扱う。
  指定がなければ`activeSpace`から比較対象の名前を取得する。
- `aidlc/spaces`直下を列挙し、`statSync().isDirectory()`が真の名前だけを追加する。
  再帰しない。通常ファイルは除くが、隠しディレクトリ、大文字、Unicode名を排除する
  名前検証はない。
- `default`は実ディレクトリがなくても必ず含める。実在する場合も重複しない。
  このため、一覧に`default`があることは、初期化済みであることを意味しない。
- 最後に名前全体をJavaScript標準の`sort()`で整列する。`default`先頭固定ではない。
- `active`は名前の完全一致で決まる。未知のcursorや空のoverrideでは、全件が
  `active: false`になることもある。
- directory列挙が失敗した場合は`default`だけを返す。
- 子のstatが途中で失敗した場合は、その時点でループ全体を終了する。
  失敗した子だけを飛ばすのではなく、それまでの収集分と`default`を整列して返す。
- statはsymlinkを追う。directoryへのリンクも一覧対象になる。壊れたリンクによる
  stat失敗も、上記の途中終了に該当する。
- 読み取りだけであり、spaceやcursorを作成・更新しない。

## 別の責務との区別

共通libに`resolveSpace`という独立関数はない。明示指定、session binding、cursorを
組み合わせる最終的な選択は`resolveWorkflowSelection`の責務である。
名前の形式と既知spaceを検証する`resolveSpaceFlag`はKnowledge専用の契約であり、
上記の共通readerへそのまま組み込むと挙動が変わる。

## Go移植の注意点

- JavaScriptの`trim()`とGoの`strings.TrimSpace`は同一ではない。
  JavaScriptはU+FEFFを除去してU+0085を保持するが、Goは逆になる。
- JavaScript標準sortはUTF-16コード単位順であり、Go文字列のバイト順とは
  一部のUnicode名で異なる。正規のASCII space名では一致する。
- 本家は読込エラーを吸収するため、`default`へのfallbackだけでは未作成と権限不足等を
  区別できない。この挙動を保持するかは、実装計画で明示したうえで承認を得る。
- `fs.FS`をproject root基準で注入すれば、標準ライブラリだけで読取処理と失敗系を
  テストできる。ただし`os.DirFS`はsymlinkによるroot外アクセスを防ぐsandboxではない。
- 今回のreaderが返した未検証の名前を、そのまま追加のファイルアクセスへ使用しない。
  名前からpathへ到達する後続の機能では、検証とfilesystem境界を別途設計する。

## 未確認・互換性の限界

- 不正UTF-8の置換単位について、本家Bun実行時とGoの厳密な同一性は未確認。
  valid UTF-8のfixtureだけから、不正なエンコードまで完全互換とは断定しない。
- stat失敗時の部分一覧は、失敗までの列挙順に依存する。本家runtimeと各OSの
  列挙順は未実測であり、Goの名前順の`ReadDir`へ置き換えて、異常時の部分集合まで
  常に一致するとは保証しない。
- 上記2点を固定する参照側テストは、今回の関連テスト調査では確認できなかった。

## 次の実装候補（未承認）

`src/internal/workspace`に共通readerの2機能だけを追加する。
CLI、space作成・切替、名前からのpath解決、intent/state/session選択は含めない。
具体的なGo API、テスト、変更ファイルは承認用計画で提示する。

## 根拠

- [共通lib](../../実装_aidlc-workflows/core/tools/aidlc-lib.ts):
  `activeSpace`（1554行付近）、`listSpaces`（2415行付近）、
  `resolveWorkflowSelection`（3611行付近）
- [Knowledge実装](../../実装_aidlc-workflows/core/tools/aidlc-knowledge.ts):
  `resolveSpaceFlag`（1162行付近）
- [cursor既定値のテスト](../../実装_aidlc-workflows/tests/unit/t160-workspace-record-resolution.test.ts)
  （114行付近）
- [空workspace一覧のテスト](../../実装_aidlc-workflows/tests/integration/t165-intent-create-p4.test.ts)
  （932行付近）
- [spaceとintentのguide](../../実装_aidlc-workflows/docs/guide/03-spaces-and-intents.md)
  （180行付近。最終的な選択優先順位は後続機能の範囲）
- Go 1.26.4のローカル標準ドキュメント: `go doc io/fs.ReadDir`、
  `go doc io/fs.Stat`、`go doc os.DirFS`。Context7のGo 1.26.0検索を先行し、
  versionの異なる抜粋や不足している詳細はローカルの公式ドキュメントで補った。

## 後続の承認

上記の未着手・未承認は調査時点の状態である。同日の次の確認で読取2機能の詳細計画が
承認され、[Issue #13](https://github.com/sori883/ai-dd/issues/13)で実装を進めることとなった。
採用したAPIと互換性の保証範囲は
[共通space読み取りの初期契約](../decisions/2026-08-31-space-reading-contract.md)に記録する。
