package update

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestDownloadLatestBinary(t *testing.T) {
	const (
		tag    = "v1.2.3"
		goos   = "linux"
		goarch = "amd64"
	)

	binaryPayload := []byte("new-binary-content")
	archiveName := fmt.Sprintf("sshelob_%s_%s_%s.tar.gz", tag, goos, goarch)
	archivePayload := mustBuildTarGzArchive(t, "sshelob", binaryPayload)

	var server *httptest.Server
	server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/repos/danilbrenner/sshelob/releases/latest":
			w.Header().Set("Content-Type", "application/json")
			_, _ = fmt.Fprintf(w, `{"tag_name":"%s","assets":[{"name":"%s","browser_download_url":"%s/download/%s"}]}`,
				tag,
				archiveName,
				server.URL,
				archiveName,
			)
		case "/download/" + archiveName:
			_, _ = w.Write(archivePayload)
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	gotTag, gotBinary, err := downloadLatestBinary(context.Background(), server.Client(), server.URL, "danilbrenner/sshelob", goos, goarch)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if gotTag != tag {
		t.Fatalf("tag mismatch: got %q, want %q", gotTag, tag)
	}
	if !bytes.Equal(gotBinary, binaryPayload) {
		t.Fatalf("binary mismatch: got %q, want %q", string(gotBinary), string(binaryPayload))
	}
}

func TestDownloadLatestBinaryErrorsWhenAssetMissing(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/repos/danilbrenner/sshelob/releases/latest" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"tag_name":"v1.2.3","assets":[]}`))
	}))
	defer server.Close()

	_, _, err := downloadLatestBinary(context.Background(), server.Client(), server.URL, "danilbrenner/sshelob", "linux", "amd64")
	if err == nil {
		t.Fatal("expected error for missing assets")
	}
}

func TestReplaceExecutable(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("replace behavior for open files differs on windows")
	}

	tmpDir := t.TempDir()
	exePath := filepath.Join(tmpDir, "sshelob")
	if err := os.WriteFile(exePath, []byte("old-content"), 0o755); err != nil {
		t.Fatalf("failed to seed executable: %v", err)
	}

	if err := replaceExecutable(exePath, []byte("new-content")); err != nil {
		t.Fatalf("replaceExecutable returned error: %v", err)
	}

	got, err := os.ReadFile(exePath)
	if err != nil {
		t.Fatalf("failed reading replaced executable: %v", err)
	}
	if string(got) != "new-content" {
		t.Fatalf("replace mismatch: got %q, want %q", string(got), "new-content")
	}
}

func TestExtractBinaryFromZip(t *testing.T) {
	var payload bytes.Buffer
	zipWriter := zip.NewWriter(&payload)
	fileWriter, err := zipWriter.Create("sshelob.exe")
	if err != nil {
		t.Fatalf("failed creating zip entry: %v", err)
	}
	if _, err := fileWriter.Write([]byte("zip-binary")); err != nil {
		t.Fatalf("failed writing zip entry: %v", err)
	}
	if err := zipWriter.Close(); err != nil {
		t.Fatalf("failed closing zip writer: %v", err)
	}

	binary, err := extractBinary("sshelob_v1.2.3_windows_amd64.zip", "sshelob.exe", payload.Bytes())
	if err != nil {
		t.Fatalf("extractBinary returned error: %v", err)
	}
	if string(binary) != "zip-binary" {
		t.Fatalf("unexpected binary payload: got %q", string(binary))
	}
}

func mustBuildTarGzArchive(t *testing.T, name string, payload []byte) []byte {
	t.Helper()

	var archive bytes.Buffer
	gzWriter := gzip.NewWriter(&archive)
	tarWriter := tar.NewWriter(gzWriter)

	header := &tar.Header{
		Name: name,
		Mode: 0o755,
		Size: int64(len(payload)),
	}
	if err := tarWriter.WriteHeader(header); err != nil {
		t.Fatalf("failed writing tar header: %v", err)
	}
	if _, err := tarWriter.Write(payload); err != nil {
		t.Fatalf("failed writing tar payload: %v", err)
	}
	if err := tarWriter.Close(); err != nil {
		t.Fatalf("failed closing tar writer: %v", err)
	}
	if err := gzWriter.Close(); err != nil {
		t.Fatalf("failed closing gzip writer: %v", err)
	}

	return archive.Bytes()
}
