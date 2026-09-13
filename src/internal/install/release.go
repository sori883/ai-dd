package install

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/sori883/ai-dd/src/harness/codex"
	"github.com/sori883/ai-dd/src/internal/release"
	"io"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"time"
)

type ReleaseOptions struct {
	Relocate                         bool
	FromProjectDir, FromBinary       string
	Root, Version, Directory, Target string
	fetch                            func(context.Context, string) ([]byte, error)
	write                            func(string, string, []byte, uint32) error
}
type ReleaseResult struct {
	Result
	Version  string
	Binaries codex.Binaries
	Pending  []string
}

func download(ctx context.Context, url string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, err
	}
	client := http.Client{Timeout: 2 * time.Minute}
	response, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK || response.ContentLength > release.MaxArchiveBytes {
		return nil, fmt.Errorf("download status/size invalid: %s", response.Status)
	}
	raw, err := io.ReadAll(io.LimitReader(response.Body, release.MaxArchiveBytes+1))
	if err != nil {
		return nil, err
	}
	if int64(len(raw)) > release.MaxArchiveBytes {
		return nil, fmt.Errorf("download too large")
	}
	return raw, nil
}
func releaseFetcher(o ReleaseOptions) func(context.Context, string) ([]byte, error) {
	if o.fetch != nil {
		return o.fetch
	}
	return func(ctx context.Context, name string) ([]byte, error) {
		if filepath.Base(name) != name || strings.ContainsAny(name, "/\\\x00") {
			return nil, fs.ErrInvalid
		}
		if o.Directory == "" {
			return download(ctx, "https://github.com/sori883/ai-dd/releases/download/"+o.Version+"/"+name)
		}
		root, err := os.OpenRoot(o.Directory)
		if err != nil {
			return nil, err
		}
		defer root.Close()
		info, err := root.Lstat(name)
		if err != nil {
			return nil, err
		}
		if !info.Mode().IsRegular() || info.Size() > release.MaxArchiveBytes {
			return nil, fmt.Errorf("invalid offline asset")
		}
		file, err := root.Open(name)
		if err != nil {
			return nil, err
		}
		defer file.Close()
		raw, err := io.ReadAll(io.LimitReader(file, release.MaxArchiveBytes+1))
		if err != nil {
			return nil, err
		}
		if int64(len(raw)) > release.MaxArchiveBytes {
			return nil, fmt.Errorf("offline asset too large")
		}
		return raw, nil
	}
}
func InstallRelease(ctx context.Context, o ReleaseOptions) (result ReleaseResult, err error) {
	if o.Target == "" {
		o.Target = runtime.GOOS + "/" + runtime.GOARCH
	}
	if !release.ValidVersion(o.Version) || !slices.Contains(release.Targets, o.Target) {
		return result, fmt.Errorf("invalid release version/target: %w", fs.ErrInvalid)
	}
	o.Root, err = filepath.EvalSymlinks(o.Root)
	if err != nil {
		return result, err
	}
	if !filepath.IsAbs(o.Root) {
		return result, fmt.Errorf("absolute project root required: %w", fs.ErrInvalid)
	}
	project, err := os.OpenRoot(o.Root)
	if err != nil {
		return result, err
	}
	defer project.Close()
	lock, err := project.OpenFile(".aidlc-install.lock", os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if err != nil {
		return result, fmt.Errorf("installation reservation: %w", err)
	}
	if err := lock.Close(); err != nil {
		project.Remove(".aidlc-install.lock")
		return result, err
	}
	defer func() { err = errors.Join(err, project.Remove(".aidlc-install.lock")) }()
	fetch := releaseFetcher(o)
	binaryData := map[string][]byte{}
	commit, goVersion := "", ""
	for _, product := range []string{"aidlc", "okf", "natural-japanese-go"} {
		manifestName, sumsName := release.MetadataNames(product)
		raw, err := fetch(ctx, manifestName)
		if err != nil {
			return result, err
		}
		sums, err := fetch(ctx, sumsName)
		if err != nil {
			return result, err
		}
		m, a, err := release.ValidateManifest(raw, sums, product, o.Version, o.Target)
		if err != nil {
			return result, err
		}
		if commit == "" {
			commit, goVersion = m.SourceCommit, m.GoVersion
		} else if commit != m.SourceCommit || goVersion != m.GoVersion {
			return result, fmt.Errorf("mixed source commit/toolchain")
		}
		archive, err := fetch(ctx, a.Archive)
		if err != nil {
			return result, err
		}
		data, err := release.ValidateBinary(archive, a)
		if err != nil {
			return result, err
		}
		binaryData[a.Binary] = data
	}
	manifestRaw, err := fetch(ctx, "aidlc-assets-manifest.json")
	if err != nil {
		return result, err
	}
	var dm release.DataManifest
	if err := release.StrictJSON(manifestRaw, &dm); err != nil {
		return result, err
	}
	if dm.Archive != "aidlc-assets_"+o.Version+".tar.gz" {
		return result, fmt.Errorf("invalid source archive name")
	}
	sums, err := fetch(ctx, "aidlc-assets-SHA256SUMS")
	if err != nil {
		return result, err
	}
	archive, err := fetch(ctx, dm.Archive)
	if err != nil {
		return result, err
	}
	entries, err := release.ValidateData(manifestRaw, sums, archive, o.Version, commit)
	if err != nil {
		return result, err
	}
	temp, err := os.MkdirTemp("", "aidlc-release-")
	if err != nil {
		return result, err
	}
	defer os.RemoveAll(temp)
	for name, data := range entries {
		p := filepath.Join(temp, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(p), 0700); err != nil {
			return result, err
		}
		if err := os.WriteFile(p, data, 0600); err != nil {
			return result, err
		}
	}
	suffix := ""
	if strings.HasPrefix(o.Target, "windows/") {
		suffix = ".exe"
	}
	binDir := filepath.Join(o.Root, "aidlc/bin", o.Version)
	result.Version = o.Version
	result.Binaries = codex.Binaries{AIDLC: filepath.Join(binDir, "aidlc"+suffix), OKF: filepath.Join(binDir, "okf"+suffix), Natural: filepath.Join(binDir, "natural-japanese-go"+suffix)}
	assets, err := codex.DistributionFrom(o.Root, result.Binaries, os.DirFS(filepath.Join(temp, "core")), os.DirFS(filepath.Join(temp, "codex")))
	if err != nil {
		return result, err
	}
	if o.Relocate {
		if !filepath.IsAbs(o.FromProjectDir) || !filepath.IsAbs(o.FromBinary) || o.FromBinary != filepath.Join(o.FromProjectDir, "aidlc/bin", o.Version, "aidlc"+suffix) {
			return result, fmt.Errorf("relocation requires exact same-version source runtime paths: %w", fs.ErrInvalid)
		}
		for name, want := range binaryData {
			path := filepath.Join("aidlc/bin", o.Version, name)
			info, e := project.Lstat(path)
			if e != nil || !info.Mode().IsRegular() {
				return result, fmt.Errorf("missing regular runtime %s", path)
			}
			file, e := project.Open(path)
			if e != nil {
				return result, e
			}
			raw, e := io.ReadAll(io.LimitReader(file, release.MaxArchiveBytes+1))
			file.Close()
			if e != nil || release.Hash(raw) != release.Hash(want) {
				return result, fmt.Errorf("runtime differs from selected release: %s", path)
			}
		}
		relocated, e := RelocateFrom(o.Root, o.FromProjectDir, result.Binaries, codex.SiblingBinaries(o.FromBinary), os.DirFS(filepath.Join(temp, "core")), os.DirFS(filepath.Join(temp, "codex")))
		result.Paths, result.Pending = relocated.Paths, relocated.Pending
		return result, e
	}
	type file struct {
		path string
		data []byte
		mode uint32
	}
	var files []file
	for _, a := range assets {
		files = append(files, file{a.Path, a.Data, 0644})
	}
	for name, data := range binaryData {
		files = append(files, file{filepath.ToSlash(filepath.Join("aidlc/bin", o.Version, name)), data, 0755})
	}
	slices.SortFunc(files, func(a, b file) int { return strings.Compare(a.path, b.path) })
	for _, f := range files {
		if err := checkDestination(project, f.path); err != nil {
			return result, err
		}
	}
	for _, f := range files {
		result.Pending = append(result.Pending, f.path)
	}
	for _, f := range files {
		if err := project.MkdirAll(filepath.Dir(f.path), 0755); err != nil {
			return result, err
		}
		var writeErr error
		if o.write != nil {
			writeErr = o.write(o.Root, f.path, f.data, f.mode)
		} else {
			output, e := project.OpenFile(f.path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, os.FileMode(f.mode))
			if e != nil {
				writeErr = e
			} else {
				_, e = output.Write(f.data)
				writeErr = errors.Join(e, output.Close())
			}
		}
		if writeErr != nil {
			if _, e := project.Lstat(f.path); e == nil {
				result.Paths = append(result.Paths, f.path)
			}
			return result, fmt.Errorf("partial installation at %s: %w", f.path, writeErr)
		}
		result.Paths = append(result.Paths, f.path)
		result.Pending = result.Pending[1:]
	}
	return result, nil
}

// Keep JSON output formatting stable for the installer command.
func EncodeReleaseResult(result ReleaseResult) ([]byte, error) {
	return json.MarshalIndent(result, "", "  ")
}
