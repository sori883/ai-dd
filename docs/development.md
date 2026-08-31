# 開発手順

## 前提

- Go 1.26以上
- Git

このmoduleはGo標準ライブラリだけを使用します。外部Go moduleや追加の開発toolを導入する場合は、先に必要性と設計理由を提示し、明示的な承認を得てください。

## ローカル検証

repository rootで次を実行します。

```sh
gofmt -w src
go test -count=1 ./...
go test -race -shuffle=on ./...
go vet ./...
go mod tidy -diff
git diff --check
```

coverageを確認する場合は、repository外の一時fileへ出力します。

```sh
coverage_file="$(mktemp)"
go test -shuffle=on -coverprofile="$coverage_file" ./...
go tool cover -func="$coverage_file"
```

### Workspaceのspace・intent readerと接続

内部readerの単体テストと、実filesystemでの統合テストを分けて実行します。

```sh
go test -count=1 ./src/internal/workspace
go test -tags=integration -count=1 ./src/internal/workspace
go test -count=1 -shuffle=on ./...
go test -race -shuffle=on ./...
go test -tags=integration -race -shuffle=on ./...
go vet -tags=integration ./...
```

単体テストは`fstest.MapFS`と最小の失敗注入stubを使い、cursor、明示override、
部分データ付きエラー、途中Stat失敗、JavaScriptのtrim・UTF-16 sort互換を固定します。
cwdや環境変数は変更しません。

intentではさらに、非空explicitの未検証pass-throughとFS未アクセス、cursor優先、
候補0・1・複数のfallback、無効cursorを正規化・Statする前の拒否、nested pathと`.`、
registry/state本文の未読を確認します。途中Stat失敗で後続も調べる点は、spaceと異なります。
`ActiveIntent`のboolは選択の有無であり、未検証explicitを後続path操作へそのまま渡してよい
という意味ではありません。

spaceの統合テストは`os.DirFS(t.TempDir())`に対して、未配置・空・通常file・directory、
directory/fileへのsymlink、broken link、root外symlinkを検証します。
intentの統合テストは、`t.TempDir()`を`os.OpenRoot`で開いた`Root.FS()`を使用します。
cursorとmarkerの双方で、内側の相対linkの受入れ、外側・絶対・壊れたlinkによるfallbackや
候補除外を検証します。reader単体テストでは`Root`のopen/closeをfixtureが管理し、
接続APIの`ReadSelection`では製品内部が管理します。
`fs.FS`自体、`os.DirFS`、`fs.Sub`にはsymlink封じ込め保証がない点に注意してください。

両readerの統合テストで、読取前後のpath、内容、mode、更新時刻、symlink先を比較し、
製品処理による作成・変更がないことを確認します。fixtureの作成は製品への作成機能追加とは別です。
Windowsでsymlink作成権限が不足する場合は、そのケースだけ理由付きでskipします。

`ReadSelection`だけを確認する場合は次を実行します。

```sh
go test -count=1 -run '^(TestReadSelection|TestLocalizeSpace)' ./src/internal/workspace
go test -tags=integration -count=1 -run '^(TestReadSelection|TestLocalizeSpace)' ./src/internal/workspace
```

接続の単体テストではroot候補の優先順位・絶対path、spaceの正規化前検証とLocalizeのOS差を
確認します。open・child-open・closeのエラーは非公開helperの関数引数から注入し、権限不足を
chmodや実行ユーザーの権限だけに依存させません。実Rootが必要な経路はintegration側でcaptureし、
正常・不在・無効space・接続失敗・Close失敗でも取得済みRootが逆順に閉じられること、
複数の原因を`errors.Is`で識別できること、error時にmetadataがzero valueになることを確認します。

実FSではdefault/custom/未知space、未作成の各階層、候補0・1・複数、cursorの優先を確認します。
projectの初回symlinkは追従しますが、childはproject内の相対linkだけを許可し、外向き・絶対linkを
接続error、壊れた相対linkを不在として扱います。`active-space`の拒否されたリンクはdefaultへ
fallbackします。さらに、同じproject内でもintents外のcursor/markerはfallback・除外となることを
固定します。接続テストも外側fixtureを含む読取前後snapshotを比較し、書込みや自動作成が
ないことを検証します。cursor・ReadDir・Statのエラー吸収は既存readerのままです。

本家ローカル`2.6.123`との差分は[接続境界と互換性への影響](architecture.md#本家との差分と影響)、
合意は[Accepted計画](ram/decisions/2026-08-31-workspace-reading-composition-plan.md)を参照してください。
将来のconsumerは`Selection`のboolを安全性・state内容・存在継続の保証とせず、名前を再利用する
path操作に独自の検証とFS境界を用意する必要があります。

既存CIのquality jobでも統合テストを実行します。ただしCIの実行OSはUbuntuであり、
6 targetのcross buildを各OSの実行証拠とは扱いません。workspace readerは公開CLIから未到達
なので、help/version smokeや配布E2Eをspace・intent・接続機能の検証証拠にはしません。

### Space作成CLI

既存projectへ新しいspaceを作ります。複数単語の名前は引用してください。

```sh
go run ./src/cmd/aidlc space create "Team Alpha" --project-dir /path/to/existing-project
```

`--project-dir path`と`--project-dir=path`はcommandの前・途中・名前の後に配置できます。
省略時のroot優先順位は`AIDLC_PROJECT_DIR`、`CLAUDE_PROJECT_DIR`、cwdです。
明示flagが最優先で、相対pathはcwd基準です。project自体は新設しません。
余剰位置引数、未知flag、重複flag、欠落・空のpath値、空の名前、作成位置の`help`・`-h`は
callback前に拒否します。分離形の次tokenが`-`始まりなら値欠落とし、例えば
`--project-dir --force`をpathとして飲み込みません。`-`始まりの実pathは
`--project-dir=-dir`または`--project-dir ./-dir`で渡せます。

成功時はstdoutに`Space created: team-alpha\n`、stderrは空、exit 0です。
認識済み`space create`の失敗はstderrのJSON 1行とexit 1、rootの未知commandは従来どおりexit 2です。
help/versionと構文エラーではcwd・環境変数・FSへアクセスしません。作成時も現在のspace/intentを
読まず、自動切替しません。本文・生成treeと境界は[Space作成](architecture.md#space作成)を参照してください。

```sh
go test -count=1 ./src/internal/cli ./src/cmd/aidlc ./src/internal/workspace
go test -tags=integration -count=1 -run '^TestCreateSpace' ./src/internal/workspace
go test -count=1 -race -shuffle=on ./...
go test -tags=integration -count=1 -race -shuffle=on ./...
go vet ./...
go vet -tags=integration ./...
```

単体テストは名前のUnicode小文字化・48文字切詰め、flag全位置と`=形式`、callback未実行、
stdout/stderr・終了コード、lazyなmainのroot入力を固定します。open/read/write/Closeの異常は
小さい関数seamから注入し、権限エラーをchmodだけに依存させません。
integrationでは7 directory・6 fileと本文、default orgだけの継承、空org、不在fallback、
既存targetの全種別と同名競合、内部・外部・絶対・broken link、部分writeと複数Close失敗を確認します。
snapshotで既存default・他space・cursorの内容・mode・mtimeを比較し、対象追加に必要な親directoryの
mtime以外を変えないこと、境界外に書き込まないことを確認します。

errorが返っても生成物が残る場合があり、再試行は既存targetとして拒否されます。
自動cleanupや修復は行わず、利用者が内容を確認して次の対応を決めます。
本家との承認済みの6つの意図的な変更は[差分表](architecture.md#space作成の意図的な差分)を参照してください。
space作成の配布E2Eはread-only APIや完全なworkspace lifecycleの検証とは区別します。

## 実行fileの確認

```sh
build_dir="$(mktemp -d)"
go build -o "$build_dir/aidlc" ./src/cmd/aidlc
"$build_dir/aidlc" --help
"$build_dir/aidlc" --version
```

linkerからbuild情報を設定する例です。

```sh
go build \
  -ldflags "-X github.com/sori883/ai-dd/src/internal/buildinfo.Version=v0.1.0 -X github.com/sori883/ai-dd/src/internal/buildinfo.Commit=abcdef0" \
  -o "$build_dir/aidlc" \
  ./src/cmd/aidlc
"$build_dir/aidlc" version
```

## Cross build

CIは`CGO_ENABLED=0`で次の6ターゲットのCLIをbuildします。

- `darwin/amd64`
- `darwin/arm64`
- `linux/amd64`
- `linux/arm64`
- `windows/amd64`
- `windows/arm64`

ローカルでは次のloopで、CLIとintegrationタグ付きworkspaceテストバイナリを確認できます。
mainはspace作成のためworkspaceをimportしますが、CLI buildはworkspaceの`_test.go`や
integrationタグ付きテストをcompileしないため、テストバイナリも別に確認します。
いずれもコンパイルの確認であり、各OSでのテスト実行とは区別します。

```sh
build_dir="$(mktemp -d)"
for target in darwin/amd64 darwin/arm64 linux/amd64 linux/arm64 windows/amd64 windows/arm64; do
  goos="${target%/*}"
  goarch="${target#*/}"
  suffix=""
  if [ "$goos" = "windows" ]; then suffix=".exe"; fi
  CGO_ENABLED=0 GOOS="$goos" GOARCH="$goarch" \
    go build -trimpath -o "$build_dir/aidlc-$goos-$goarch$suffix" ./src/cmd/aidlc
  CGO_ENABLED=0 GOOS="$goos" GOARCH="$goarch" \
    go test -c -tags=integration -o "$build_dir/workspace-$goos-$goarch.test$suffix" ./src/internal/workspace
done
```

## 配布E2E

repository外の承認済みlocal sandboxへ実行物を配置して確認する手順と安全規則は、
[配布E2Eテスト](e2e-testing.md)を参照してください。実行ごとの証跡は
[`e2e-runs/`](e2e-runs/)へ記録します。

## 変更手順

1. GitHub Issueと承認済み計画でscopeを固定します。
2. observable behaviorを表す失敗testを追加してREDを確認します。
3. 最小実装でGREENにし、testがgreenの間だけrefactorします。
4. 上記のローカル検証を実行します。
5. Issueへ紐づくPRを作成します。自動mergeは行いません。
