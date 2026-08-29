# 開発手順

## 前提

- Go 1.26以上
- Git

このmoduleはGo標準ライブラリだけを使用します。外部Go moduleや追加の開発toolを導入する場合は、先に必要性と設計理由を提示し、明示的な承認を得てください。

## ローカル検証

repository rootで次を実行します。

```sh
gofmt -w src
go test ./...
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

CIは`CGO_ENABLED=0`で次の6ターゲットをbuildします。

- `darwin/amd64`
- `darwin/arm64`
- `linux/amd64`
- `linux/arm64`
- `windows/amd64`
- `windows/arm64`

ローカルでも次のloopで同じ対象を確認できます。

```sh
build_dir="$(mktemp -d)"
for target in darwin/amd64 darwin/arm64 linux/amd64 linux/arm64 windows/amd64 windows/arm64; do
  goos="${target%/*}"
  goarch="${target#*/}"
  suffix=""
  if [ "$goos" = "windows" ]; then suffix=".exe"; fi
  CGO_ENABLED=0 GOOS="$goos" GOARCH="$goarch" \
    go build -trimpath -o "$build_dir/aidlc-$goos-$goarch$suffix" ./src/cmd/aidlc
done
```

## 変更手順

1. GitHub Issueと承認済み計画でscopeを固定します。
2. observable behaviorを表す失敗testを追加してREDを確認します。
3. 最小実装でGREENにし、testがgreenの間だけrefactorします。
4. 上記のローカル検証を実行します。
5. Issueへ紐づくPRを作成します。自動mergeは行いません。
