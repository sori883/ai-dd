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

### Workspaceのspace・intent reader

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
候補除外を検証します。`Root`のopen/closeはfixtureの責務で、製品のspace接続処理は未実装です。
`fs.FS`自体、`os.DirFS`、`fs.Sub`にはsymlink封じ込め保証がない点に注意してください。

両readerの統合テストで、読取前後のpath、内容、mode、更新時刻、symlink先を比較し、
製品処理による作成・変更がないことを確認します。fixtureの作成は製品への作成機能追加とは別です。
Windowsでsymlink作成権限が不足する場合は、そのケースだけ理由付きでskipします。

既存CIのquality jobでも統合テストを実行します。ただしCIの実行OSはUbuntuであり、
6 targetのcross buildを各OSの実行証拠とは扱いません。workspace readerは公開CLIから未到達
なので、help/version smokeや配布E2Eをspace・intent機能の検証証拠にはしません。

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
CLIはworkspaceをimportしないため、CLI buildだけではreaderのcross build証拠になりません。
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
