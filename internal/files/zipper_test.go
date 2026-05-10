package files

import (
	"archive/zip"
	"bytes"
	"crypto/md5"
	"encoding/hex"
	"io"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/cerebriumai/cerebrium/pkg/projectconfig"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestCreateZip_Reproducible verifies the central guarantee of the deterministic
// zip work: two CreateZip calls on byte-identical content produce byte-identical
// zip output, which means the same content uploaded by two different users
// (or by the same user twice) gets the same S3 ETag and therefore the same
// fingerprint in the backend's app-build-fingerprints table.
//
// The test deliberately mutates each source file's mtime between the two zip
// builds to prove that filesystem mtimes don't leak into the zip output.
func TestCreateZip_Reproducible(t *testing.T) {
	srcDir := t.TempDir()
	mustWriteFile(t, filepath.Join(srcDir, "main.py"), "print('hello')\n")
	mustWriteFile(t, filepath.Join(srcDir, "lib", "util.py"), "x = 1\n")
	mustWriteFile(t, filepath.Join(srcDir, "cerebrium.toml"), "[cerebrium]\nname = \"test\"\n")

	cfg := &projectconfig.ProjectConfig{
		Dependencies: projectconfig.DependenciesConfig{
			Pip: map[string]string{
				"numpy":    "1.24.0",
				"requests": ">=2.28.0",
			},
			Apt: map[string]string{
				"ffmpeg": "latest",
			},
		},
	}

	hashA := buildZipHash(t, srcDir, cfg)

	// Bump mtime on every source file so we'd be detected if mtime were
	// leaking into the zip output.
	bumped := time.Now().Add(2 * time.Hour)
	mustWalkSetMtime(t, srcDir, bumped)

	hashB := buildZipHash(t, srcDir, cfg)

	assert.Equal(t, hashA, hashB,
		"CreateZip output should be byte-identical across runs, even after touching source mtimes")
}

// TestCreateZip_PinsModifiedTimeToEpoch confirms every entry in the produced
// zip has the fixed epoch as its modified time — both project files (which
// previously inherited filesystem mtime) and dependency files (which previously
// got time.Now()).
func TestCreateZip_PinsModifiedTimeToEpoch(t *testing.T) {
	srcDir := t.TempDir()
	mustWriteFile(t, filepath.Join(srcDir, "main.py"), "x = 1\n")

	cfg := &projectconfig.ProjectConfig{
		Dependencies: projectconfig.DependenciesConfig{
			Pip: map[string]string{"numpy": "1.24.0"},
		},
	}

	zipPath := buildZip(t, srcDir, cfg)
	r, err := zip.OpenReader(zipPath)
	require.NoError(t, err)
	defer r.Close()

	require.NotEmpty(t, r.File, "zip should contain at least one entry")
	for _, f := range r.File {
		assert.True(t, f.Modified.Equal(zipEpoch),
			"entry %q has Modified=%s, want %s", f.Name, f.Modified, zipEpoch)
	}
}

// TestAddDependencyFiles_DeterministicOrder asserts dep files are written in
// sorted order regardless of map iteration order. Run multiple times to make
// the randomized map order surface if the sort were removed.
func TestAddDependencyFiles_DeterministicOrder(t *testing.T) {
	deps := map[string]string{
		"requirements.txt":  "numpy==1.0.0\n",
		"pkglist.txt":       "ffmpeg\n",
		"conda_pkglist.txt": "pandas==2.0.0\n",
	}
	wantOrder := []string{"conda_pkglist.txt", "pkglist.txt", "requirements.txt"}

	for i := 0; i < 10; i++ {
		var buf bytes.Buffer
		zw := zip.NewWriter(&buf)
		require.NoError(t, AddDependencyFiles(zw, deps))
		require.NoError(t, zw.Close())

		r, err := zip.NewReader(bytes.NewReader(buf.Bytes()), int64(buf.Len()))
		require.NoError(t, err)

		got := make([]string, 0, len(r.File))
		for _, f := range r.File {
			got = append(got, f.Name)
		}
		assert.Equal(t, wantOrder, got, "iteration %d", i)
	}
}

func buildZipHash(t *testing.T, srcDir string, cfg *projectconfig.ProjectConfig) string {
	t.Helper()
	zipPath := buildZip(t, srcDir, cfg)
	f, err := os.Open(zipPath)
	require.NoError(t, err)
	defer f.Close()

	h := md5.New()
	_, err = io.Copy(h, f)
	require.NoError(t, err)
	return hex.EncodeToString(h.Sum(nil))
}

func buildZip(t *testing.T, srcDir string, cfg *projectconfig.ProjectConfig) string {
	t.Helper()

	prevWD, err := os.Getwd()
	require.NoError(t, err)
	require.NoError(t, os.Chdir(srcDir))
	t.Cleanup(func() { _ = os.Chdir(prevWD) })

	fileList, err := DetermineIncludes([]string{"**/*"}, nil)
	require.NoError(t, err)
	// Sort to match what the deploy command does in practice and keep this
	// test from depending on filepath.Walk's (already deterministic) order.
	sortStrings(fileList)

	zipPath := filepath.Join(t.TempDir(), "out.zip")
	_, err = CreateZip(fileList, zipPath, cfg)
	require.NoError(t, err)
	return zipPath
}

func mustWriteFile(t *testing.T, path, content string) {
	t.Helper()
	require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o755))
	require.NoError(t, os.WriteFile(path, []byte(content), 0o644))
}

func mustWalkSetMtime(t *testing.T, root string, ts time.Time) {
	t.Helper()
	require.NoError(t, filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return err
		}
		return os.Chtimes(path, ts, ts)
	}))
}

func sortStrings(s []string) {
	// tiny inline sort to keep the test self-contained.
	for i := 1; i < len(s); i++ {
		for j := i; j > 0 && s[j-1] > s[j]; j-- {
			s[j-1], s[j] = s[j], s[j-1]
		}
	}
}
