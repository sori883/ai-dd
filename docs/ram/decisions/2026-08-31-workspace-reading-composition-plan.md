# Workspace読み取り接続の実装計画

- 日付: 2026-08-31
- 状態: Accepted
- GitHub Issue: [#17](https://github.com/sori883/ai-dd/issues/17)
- 前提: [intent読み取りのPR #16](https://github.com/sori883/ai-dd/pull/16)はマージ済み、
  [Issue #15](https://github.com/sori883/ai-dd/issues/15)はClosed
- 関連: [root解決](2026-08-30-project-root-resolution.md)、
  [space読み取り](2026-08-31-space-reading-contract.md)、
  [intent読み取り](2026-08-31-intent-reading-plan.md)、
  [実装順序](2026-08-31-internal-workspace-before-status.md)

## 承認状況

root・space・intentの読み取りをつなぐ内部処理を次の候補とし、
「まず、この接続部分の詳細計画を作る進め方でよいですか？」と確認した。
ユーザーは「はい。」と回答した。この回答は詳細計画を作る了承であり、
以下の新しい境界・エラー契約や実装への承認とは扱わない。

計画提示時点ではこの記録とRAM索引だけを更新し、実装コード、設定、Issue、PRは変更しなかった。
既存のAccepted記録を置換せず、readerの呼出側として新しい契約を提案した。

### 詳細計画の承認（2026-08-31）

この計画へのリンクと、内部APIを1つ追加する範囲、space名・参照範囲の制約、
不在とその他接続エラーの区別、内部Close責務、標準ライブラリのみの方針を提示した。
Issue作成からTDD、独立レビュー、PR作成まで進めてよいか確認し、ユーザーは「はい。」と回答した。
以下を承認済み実装契約として採用し、Issue #17を作成した。自動マージはしない。

同時にユーザーから、今後は本家実装との差分を自発的に提示し、その運用をプロジェクトの
`AGENTS.md`へ記載する指示を受けた。親担当の文書変更としてこのタスクに含め、
別の[差分提示ルール](2026-08-31-upstream-difference-reporting.md)へ方針と理由を記録する。

## 目的・確認済みの事実

`src/internal/workspace`に実装済みの`ResolveRoot`、`ActiveSpace`、
`ListIntentDirs`、`ActiveIntent`を、現在選択された1 spaceについて接続する。

- 本家の参照対象はローカルAI-DLC `2.6.123`。共通helperはspace名のslug検証や
  存在確認を行わない。Knowledge/sessionにある別用途の名前・所属検証は流用しない。
- 既存`ResolveRoot`はfilesystemへアクセスしない。候補の優先順位と正規化は維持する。
- `ActiveSpace`は読取エラー・空・欠損を`default`に置換する。
  `ActiveIntent`と`ListIntentDirs`も操作ごとの読取エラーを吸収する契約である。
- intent readerへ実FSを渡す際の`os.Root.FS()`採用は承認済みだが、
  projectから対象directoryを開く処理と、その失敗・Closeの扱いは未実装である。
- 公開CLIはまだworkspaceを呼ばない。この追加だけで利用者向けコマンドは増えない。

## 承認済みの最小API

```go
func ReadSelection(input RootInput) (Selection, error)
```

`Selection`は`ProjectRoot`、`SpaceName`、`IntentDirs []string`、`ActiveIntent`、
`HasActiveIntent`を持つmetadataのみの結果とする。pathと名前はstring、選択の有無はbool。
`Root`や`fs.FS`は返さず、すべてのClose責務をこの関数の内部で完結させる。
成功時の空一覧はnon-nil slice、error時は`Selection`のzero valueを返す。

処理順は以下とする。

1. 既存`ResolveRoot`へ入力を渡し、結果が絶対pathであることを確認する。
2. `os.OpenRoot`でprojectを開き、`ActiveSpace(projectRoot.FS())`を一度呼ぶ。
3. 返されたspace名を下記規則で検証・OSのpathへ変換する。
4. 開いたprojectRootから相対的に`aidlc/spaces/<space>/intents`を`OpenRoot`する。
5. `ListIntentDirs`と`ActiveIntent(intentsRoot.FS(), "")`を呼ぶ。
6. intentsRoot、projectRootの逆順でCloseし、結果を返す。

processのcwdや環境変数を内部で読まない。space/intentの明示override引数、
space一覧、store層、独自FS interfaceは追加しない。Go標準ライブラリだけを使用する。

## 今回承認された接続契約

### 名前と参照範囲

- space名は`fs.ValidPath`を満たす1 componentに限定する。名前が`.`の場合と、slashを含む場合は拒否する。
  `../other`、`a/../b`、`nested/name`、絶対path等を正規化前に拒否する。
  既存readerによるJS相当のtrim後に、接続側で追加のtrimやCleanは行わない。
- その後`filepath.Localize`でOSのpathへ変換する。Windowsでbackslash等をseparatorに
  再解釈しない。大文字・Unicode・underscore等を共通のASCII slug規則で拒否せず、
  OSごとに表現できない名前はエラーとする。全OSで作成可能な名前の完全一致は保証しない。
- 無効名は`default`へ置換せず接続エラーとする。安全な形式の未知名については
  space一覧への所属を要求せず、directory不在なら下記の空結果にする。
- 初回`os.OpenRoot`が開いたprojectを信頼境界とする。指定されたproject自体がsymlinkの
  場合は追従する。子directoryはこのRootから相対的に開き、project外への逸脱と
  絶対symlinkを拒否する。project内の別directoryへの相対symlinkは許可する。
- その後のintent読取りは、開いたintentsRootの内側に限定する。
  project内に留まっていてもintentsRoot外を指すcursor/markerのリンクは拒否され、
  既存readerのfallback・候補除外となる。

これは本家が許す任意のpath結合・symlink追従より狭い接続契約である。
既存reader単体の名前やエラーの契約は変えず、新しい接続APIに適用する。
childの絶対pathを組み立てて別の`os.OpenRoot`へ渡す方式や、
`os.DirFS`・`fs.Sub`・path文字列のprefix確認による封じ込めは採用しない。

### 不在・エラー・Close

| 状況 | 結果 |
| --- | --- |
| 解決結果が相対path、space検証・Localize失敗 | `fs.ErrInvalid`を識別できるerror。無効spaceではchildを開かない |
| projectのOpenRoot失敗 | 不在を含め、すべてerror。低優先順位のrootへ切り替えない |
| `active-space`読取失敗・欠損・空 | 既存readerどおり`default`を使う |
| child OpenRootが`errors.Is(err, fs.ErrNotExist)` | 正常な空一覧・現在intent未選択。解決したrootとspace名を保持する |
| childのその他open失敗 | 権限不足、directoryでない、リンク越境等の原因を包んでerrorを返す |
| open後のcursor・ReadDir・Stat失敗 | 既存readerのfallback・除外契約を維持する |
| Close失敗 | errorを返し、先行エラーや他のClose失敗も`errors.Join`で保持する |

childの不在条件には、祖先directoryの未作成や壊れた相対symlinkも含まれ得る。
「未初期化と診断できた」とは扱わず、あくまで標準error分類に基づく空結果とする。
自動作成・修復はしない。成功して取得したすべてのRootを各経路でCloseし、
nil FSをreaderへ渡さない。Close失敗は、空結果として返せる経路でもerrorにする。

I/O errorは操作と対象の文脈を付けて`%w`で保持し、OS固有のmessage文字列で判定しない。
既存reader内で吸収するエラーは接続側からも観測できないため、
このAPIがすべての権限不足やI/O障害を通知するとは説明しない。

## 対象ファイル・所有権・対象外

実装writerは`go_tdd_implementer`の1名とする。

- 新規`src/internal/workspace/selection.go`: 接続API、結果型、最小の非公開helper。
- 新規`src/internal/workspace/selection_test.go`: 入力検証と失敗分岐の単体テスト。
- 新規`src/internal/workspace/selection_integration_test.go`: `integration`タグ付き実FSテスト。
- 更新`docs/architecture.md`、`docs/development.md`: 接続境界・互換差分・検証方法。

既存root/space/intentのGo実装・APIの変更は対象外とする。RAMの承認・索引、GitHub操作は
親担当とし、追加指示に基づく`AGENTS.md`の差分提示ルールも親が更新する。
実装担当との編集期間は重ねない。既存CIでintegrationテストを実行できるため、
CI設定変更、外部module・toolの導入は含めない。

CLI/status、space一覧、明示override、registry/session、state本文の解釈、作成・切替・削除、
保存形式変更、配布E2Eは対象外。利用者dataへの変更はない。

## TDDと検証計画

1. root入力の受渡し・優先順位と絶対pathの確認。不在の明示rootを別候補で救済しない。
2. default/custom spaceを読み分け、候補0・1・複数とcursorの選択結果を返す。
3. spaceの1 component検証、正規化前拒否、LocalizeのOS差を固定する。
   無効名でchildへのアクセスをしないことも確認する。
4. childの不在とその他open failureを分け、標準error分類と原因の保持を検証する。
5. 正常、無効space、child失敗、Close失敗の各経路で取得済みRootを閉じる。
   複数エラーの保持とerror時のzero valueを確認する。
6. 実FSでproject内の相対リンク、project外・絶対・壊れたリンク、intents境界を検証する。
   読取前後のsnapshotを比較し、作成・変更がないことを確認する。

open・child-open・closeの異常系は、非公開helperへの関数引数注入で再現する。
mutable globalや独自FS/store interfaceは追加しない。権限テストをchmodだけに依存させず、
実Rootが必要なcaseはintegration側でcaptureして、帰還後に閉じていることも確認する。
Windowsでsymlink作成権限がない場合は該当caseだけ理由付きでskipする。
各追加動作についてRED→GREENを記録し、既にGREENのガードをRED実績には数えない。

以下は実装後の検証予定であり、新機能について実行済みの証拠ではない。

```sh
go test -count=1 ./src/internal/workspace
go test -tags=integration -count=1 ./src/internal/workspace
go test -count=1 -shuffle=on ./...
go test -race -shuffle=on ./...
go test -tags=integration -race -shuffle=on ./...
go vet ./...
go vet -tags=integration ./...
go mod tidy -diff
gofmt -l src
git diff --check
```

既存のdarwin/linux/windows × amd64/arm64の6構成では、`CGO_ENABLED=0`のCLI buildに加え、
`go test -c -tags=integration`でworkspaceテストもcross compileする。
CLIはworkspaceをimportしないため、CLI buildやhelp/version smokeだけを機能検証にしない。
native実行、Ubuntu CI実行、他OSのコンパイル確認は区別して報告する。

## 代替案・残余リスク・復旧

- space等をさらに段階的にRootとして開く案は、階層ごとの越境も制限できるが、
  project内の相対リンクで使える配置を追加制限し、Rootの管理も増やすため今回は採用しない。
- Rootを引数・返値へ公開する案は、使い回せる一方でClose責務とpathの対応を呼出側へ
  持ち出す。今回の単発metadata読取りでは採用しない。
- readerは別々に読むため、並行更新中に一覧と現在intentが一貫したsnapshotになる保証はない。
  boolは選択の有無であり、state内容の妥当性や返却後も存在することの保証ではない。
- `os.Root`はmountや特殊file/deviceまで遮断する完全なsandboxではない。
  不正UTF-8、OSごとの名前制約、Node/Bunのpath解釈までの完全互換は保証しない。
- 撤回時は追加接続処理・テスト・対応文書を取り消す。利用者dataのmigrationは不要。

## 根拠と次のゲート

- 既存Go実装: `src/internal/workspace/root.go`、`space.go`、`intent.go`。
- 本家: `docs/実装_aidlc-workflows/core/tools/aidlc-lib.ts`の`activeSpace`、`intentsDir`、
  `listIntentDirs`、`activeIntent`。`validSpaceFlag`、`resolveWorkflowSelection`、
  `aidlc-knowledge.ts`の`resolveSpaceFlag`は、流用しない別契約として比較した。
- 今回の比較対象である`aidlc-lib.ts`は、実装側の`core/tools/`、正規生成物の
  `dist/codex/.codex/tools/`、配置版`docs/配布_ai-dlc/.codex/tools/`の3者で同一内容だった。
  `aidlc-version.ts`も実装側と配置版で同一であることを確認した。
  これはローカル`2.6.123`の対象ファイルの比較であり、配布物全体の一致や最新upstreamの検証ではない。
- Go API: Context7を優先参照した上で、Go `1.26.4`同梱doc・sourceで
  [os.Root](https://pkg.go.dev/os@go1.26.4#Root)、
  [os.OpenRoot](https://pkg.go.dev/os@go1.26.4#OpenRoot)、
  [filepath.Localize](https://pkg.go.dev/path/filepath@go1.26.4#Localize)、
  [fs.ValidPath](https://pkg.go.dev/io/fs@go1.26.4#ValidPath)を確認した。
  Context7にはmaster由来の結果も含まれたため、すべてをversion固定の根拠とは扱っていない。

project単位の参照境界、space名制約、child不在とその他エラーの区別、内部Close責務を含む
この詳細計画は明示承認済みである。RAMへ回答を記録し、Issue #17を作成した。
単独TDD実装、独立レビュー、最終検証、Issueに紐づくPR作成へ進む。自動マージはしない。
