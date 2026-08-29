# Suggested Commands
- Repository inspection: `git status --short --branch`, `git diff --check`.
- GitHub repository: `gh repo view`; authenticated `gh` is available.
- Once a Go module exists: targeted iteration with `go test <changed-packages>`; completion with `go test ./...` and `go test -race ./...`.
- Format changed Go files with `gofmt -w <files>`.
- Check Serena memory references from repository root with `serena memories check`.