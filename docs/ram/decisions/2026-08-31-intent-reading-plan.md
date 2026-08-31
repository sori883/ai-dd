# Intent読み取りの実装計画

- 日付: 2026-08-31
- 状態: Accepted
- GitHub Issue: [#15](https://github.com/sori883/ai-dd/issues/15)
- 前提: [space読み取りのPR #14](https://github.com/sori883/ai-dd/pull/14)はマージ済み
- 関連: [実装順序の合意](2026-08-31-internal-workspace-before-status.md)、
  [参照契約の調査](../research/2026-08-31-intent-reading-contracts.md)

## 承認状況

ユーザーは、intent候補列挙・現在intent解決の概略と「まず詳細計画を提示し、承認後に実装」
という提案に対し、「はい、じゃあえっと作業お願いしていいですか。」と回答した。
この回答を次の作業を具体化する了承として記録する。

計画提示時点では、以下のFS境界と安全性のための互換差分は未承認の提案として記録し、
実装コード、設定、Issue、PRは変更しなかった。

### 詳細計画の承認（2026-08-31）

この計画へのリンクと、2readerだけの範囲、cursorの相対path制限、`os.Root.FS()`による
リンク制約、IssueからTDD・独立レビュー・PRまでの進め方を提示した。
ユーザーは「はい、じゃちょっとその実装お願いしてもいいですか。」と明示承認した。
以下を承認済みの実装契約として採用する。自動マージはしない。

## 目的と最小境界

本家AI-DLC `2.6.123`の`listIntentDirs`と`activeIntent`を基準に、選択済みの1 spaceの
`intents` directoryを読む2つの内部APIを`src/internal/workspace`へ追加する。

承認されたAPIは次のとおり。

```go
func ListIntentDirs(intentsFS fs.FS) []string
func ActiveIntent(intentsFS fs.FS, explicit string) (string, bool)
```

`intentsFS`の基準位置は`aidlc/spaces/<選択済みspace>/intents/`であり、project rootではない。
呼出側がその位置を基準とする非nilのFSを渡す。readerは`active-space`を読まず、
space名をpathへ変換しない。`bool`は選択された値の有無であり、存在や安全性の証明ではない。

Go標準ライブラリと関数引数による手動DIだけを使う。新しいstore層、独自FS interface、
外部module・toolは追加しない。既存のroot・space readerの動作は変更しない。

## 受入契約

### 候補列挙

1. FSの直下entryごとに`<entry>/aidlc-state.md`をStatし、存在するものを候補にする。
   markerを通常fileに限定せず、directoryや、供給FSが追従可能なsymlinkも含める。
2. 命名形式は強制しない。現行の`YYMMDD-label`と旧形式をともに扱う。
3. JavaScriptと同じUTF-16コード単位順で整列する。再帰列挙はしない。
4. ReadDirにerrorがあれば部分entriesも使わず空一覧を返す。
   個別markerのStatにerrorがあれば、その候補だけを除外し後続も調べる。
5. `intents.json`やstate本文は読まない。markerの存在はworkflow内容の妥当性を意味しない。

### 現在intentの解決

1. `explicit != ""`なら、trim・存在・名前の検証をせずそのまま返して`true`とする。
   空白だけの値やpath風の値も同じで、この場合FSへ一切アクセスしない。
2. 空explicitの場合は`active-intent`を読み、既存space readerと同じJS相当のtrimを適用する。
3. trim後の値が下記path制約を満たし、その結合先markerが存在すれば、その値を返す。
   一覧に含まれることは要求しない。
4. cursorが不在・空・read error・無効path・marker不在・Stat errorの場合、候補を列挙する。
   1件だけならそれを選び、0件または複数なら`("", false)`を返す。
   read errorと同時に返された部分dataは使わない。
5. ファイルの作成、書込、修復、選択cursorの更新はしない。

非空explicitのpass-throughは本家の契約を維持する。ここで返された値を後続の機能がpathへ
変換する場合、その機能に入力検証と参照範囲の制限が必要である。

## 承認済みの互換差分

本家が許容する任意のpath結合を再現せず、cursorには`fs.ValidPath`で有効な相対pathだけを
許可する。無効な値はmarkerへのアクセス前に拒否し、通常の候補1件fallbackへ進める。

- `nested/name`は許可する。特殊値`.`も許可し、FS rootの`aidlc-state.md`が存在すれば
  一覧に含まれなくても`.`を選択できる。
- `../other`、`a/../b`、`/absolute`、空componentを含むpath等は拒否する。
- 検証前に`Clean`して不正な値を正規化しない。検証後のmarker path組立で`.`を正規化しても、
  返す値はtrim後のraw値とする。
- backslashとcolonは`fs.FS`の名前文字として扱い、独自にpath区切りへ変換しない。
  各OSで作成可能な名前や、Node/BunのOS別path解釈まで完全一致は保証しない。

実ファイルシステムの安全な供給方法として`os.Root.FS()`を使用する。
このbackendは越境symlink・絶対symlinkを拒否するため、本家が追従できるリンクでも
read/Stat errorとなり、上記fallbackまたは候補除外になることをテストする。

`fs.FS`という型自体にはsymlink封じ込め保証がなく、任意のFSを渡したreader単体が
安全になるわけではない。`os.DirFS`や`fs.Sub`をsandboxとは扱わない。
`os.Root`もmount、特殊file、device fileまで遮断する完全なsandboxではない。

## 今回含めないもの

- project rootからspaceを決定し、対象intents directoryを安全に開く接続処理。
- FSの生成・Close、intents未作成や権限不足による`OpenRoot`失敗の上位での扱い。
  今回は、空のFSや不在errorを返すFSに対するreaderの動作を固定する。
- `intents.json`との突き合わせ、state本文解析、session binding、最終workflow選択。
- space/intentの作成・切替・削除、初期化、移行、公開CLI、status、配布E2E。

そのため、今回だけでは既存space readerやCLIからintent readerへは接続しない。
本家のhelperがexplicit返却前にも行うspace解決は、後続の接続処理へ分離する。

## 対象ファイルと所有権

実装writerは`go_tdd_implementer`の1名とする。

- 新規`src/internal/workspace/intent.go`: 2reader。既存JS trim述語を再利用する。
- 新規`src/internal/workspace/intent_test.go`: MapFS、最小の失敗注入・アクセス追跡stub。
- 新規`src/internal/workspace/intent_integration_test.go`: `integration`タグ付き実FSテスト。
- `docs/architecture.md`、`docs/development.md`: API境界、非互換、検証手順を追記する。

RAMの承認記録・索引とGitHub操作は親担当とし、実装担当と編集期間を重ねない。
既存CIはworkspaceのintegrationテストを実行するため、CI設定変更は不要とする。

## TDDと検証

次の観測可能な振る舞いを小さく分け、対象テストのRED→GREENを記録する。

1. 候補0・1・複数、markerなし・directory、現行名・旧名、ASCII・非BMP順序。
   ReadDir失敗と、途中のStat失敗後にも後続の候補が残ること。
2. explicit優先・空文字fallback・FS未読、cursor優先、stale・blank・read error、
   候補1件へのfallbackと複数時の未選択。
3. JS trim、nested path・`.`、traversal拒否、検証前Cleanなし、無効cursor先のStat未実行。
   registryとstate本文を読まないこと。
4. `t.TempDir()`と`os.Root.FS()`で、内側の相対リンク、外側・絶対・壊れたリンクを
   cursorとmarkerの双方について検証する。読取前後のsnapshotで無変更を確認する。

Windowsでsymlink作成権限が不足する場合はそのcaseのみ理由付きでskipし、それ以外の
予期しない失敗を隠さない。追加時点で既にGREENのガードテストを、新たなRED実績とは数えない。

主な検証コマンドは次のとおり。これは実装後の計画であり、実行済みの証拠ではない。

```sh
go test -count=1 ./src/internal/workspace
go test -tags=integration -count=1 ./src/internal/workspace
go test -count=1 -shuffle=on ./...
go test -race -shuffle=on ./...
go test -tags=integration -race -shuffle=on ./...
go vet -tags=integration ./...
go mod tidy -diff
gofmt -l src
git diff --check
```

darwin/linux/windows × amd64/arm64の6構成では、`CGO_ENABLED=0`のCLI buildに加え、
`go test -c -tags=integration`でworkspaceテストバイナリもコンパイルする。
CLIはまだworkspaceをimportしないため、CLI buildだけを今回の機能のcross build証拠にしない。
ネイティブ実行、CIのLinux実行、Windows等のコンパイル確認は区別して報告する。

## リスク・代替案・復旧

- エラー吸収により未作成と権限不足等を区別できない点、explicitが未検証である点を文書化する。
- 不正UTF-8の完全互換、並行更新中の一貫したsnapshotは保証しない。
- project基準のFSとspace overrideを同時に扱う案は、空spaceやspace名のpath化まで広がるため
  今回は採用しない。安全な接続処理を省略してよいという意味ではない。
- 保存形式・依存関係・公開CLI・利用者dataは変更しない。撤回時は追加reader・テスト・
  対応文書を取り消せばよく、利用者dataのmigrationや復旧処理は不要。

## 次のゲート

この最小FS境界と、cursorの相対path制限・`os.Root.FS()`採用時のリンク互換差分について、
ユーザーの明示承認を得た。親が承認を記録し、Issue #15を作成した。
単独TDD実装、独立レビュー、最終検証、Issueに紐づくPR作成まで進め、自動マージはしない。
