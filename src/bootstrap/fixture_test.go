package bootstrap

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"encoding/base64"
	"encoding/binary"
	"os"
	"os/exec"
	"testing"
	"unicode/utf16"
)

func unsafeInstaller(t *testing.T, binary []byte, mode string) []byte {
	t.Helper()
	var b bytes.Buffer
	g := gzip.NewWriter(&b)
	w := tar.NewWriter(g)
	for i := 0; i < 2; i++ {
		h := &tar.Header{Name: "aidlc-install", Typeflag: tar.TypeReg, Mode: 0755, Size: int64(len(binary))}
		if mode == "symlink" {
			h.Typeflag = tar.TypeSymlink
			h.Linkname = "outside"
			h.Size = 0
		}
		if err := w.WriteHeader(h); err != nil {
			t.Fatal(err)
		}
		if h.Size > 0 {
			w.Write(binary)
		}
		if mode == "symlink" {
			break
		}
	}
	w.Close()
	g.Close()
	return b.Bytes()
}

// The wrapper uses the public completed-download invocation verbatim and writes
// a sentinel only after control returns. Environment values avoid command quoting.
func powerShellBootstrapCommand(shell, script, version, project, invocation, sentinel string) *exec.Cmd {
	if invocation == "file" {
		return exec.Command(shell, "-NoProfile", "-NonInteractive", "-File", script, version, project)
	}
	source := `$script = [System.IO.File]::ReadAllText($env:AI_DD_TEST_SCRIPT)
& ([scriptblock]::Create($script)) $env:AI_DD_TEST_VERSION $env:AI_DD_TEST_PROJECT
$code = $LASTEXITCODE
[System.IO.File]::WriteAllText($env:AI_DD_TEST_SENTINEL, [string]$code)
exit $code`
	units := utf16.Encode([]rune(source))
	raw := make([]byte, 2*len(units))
	for i, u := range units {
		binary.LittleEndian.PutUint16(raw[i*2:], u)
	}
	cmd := exec.Command(shell, "-NoProfile", "-NonInteractive", "-EncodedCommand", base64.StdEncoding.EncodeToString(raw))
	cmd.Env = append(os.Environ(), "AI_DD_TEST_SCRIPT="+script, "AI_DD_TEST_VERSION="+version, "AI_DD_TEST_PROJECT="+project, "AI_DD_TEST_SENTINEL="+sentinel)
	return cmd
}

// Capture host diagnostics separately from the installer's JSON output.
func bootstrapCommandOutput(cmd *exec.Cmd) ([]byte, []byte, error) {
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	return stdout.Bytes(), stderr.Bytes(), err
}
