package update

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"runtime"
	"strings"
)

type releaseResponse struct {
	TagName string         `json:"tag_name"`
	Assets  []releaseAsset `json:"assets"`
}

type releaseAsset struct {
	Name               string `json:"name"`
	BrowserDownloadURL string `json:"browser_download_url"`
}

func RunUpdate(ctx context.Context, client *http.Client, apiBaseURL, repo string) (string, error) {
	if client == nil {
		client = http.DefaultClient
	}

	tag, binary, err := downloadLatestBinary(ctx, client, apiBaseURL, repo, runtime.GOOS, runtime.GOARCH)
	if err != nil {
		return "", err
	}

	exePath, err := os.Executable()
	if err != nil {
		return "", fmt.Errorf("resolve current executable path: %w", err)
	}

	if err := replaceExecutable(exePath, binary); err != nil {
		return "", err
	}

	return tag, nil
}

func downloadLatestBinary(ctx context.Context, client *http.Client, apiBaseURL, repo, goos, goarch string) (string, []byte, error) {
	release, err := fetchLatestStableRelease(ctx, client, apiBaseURL, repo)
	if err != nil {
		return "", nil, err
	}

	asset, err := selectReleaseAsset(release, goos, goarch)
	if err != nil {
		return "", nil, err
	}

	payload, err := fetchURL(ctx, client, asset.BrowserDownloadURL)
	if err != nil {
		return "", nil, fmt.Errorf("download release asset %q: %w", asset.Name, err)
	}

	binaryName := "sshelob"
	if goos == "windows" {
		binaryName = "sshelob.exe"
	}

	binary, err := extractBinary(asset.Name, binaryName, payload)
	if err != nil {
		return "", nil, err
	}

	return release.TagName, binary, nil
}

func fetchLatestStableRelease(ctx context.Context, client *http.Client, apiBaseURL, repo string) (*releaseResponse, error) {
	endpoint := strings.TrimRight(apiBaseURL, "/") + "/repos/" + repo + "/releases/latest"
	payload, err := fetchURL(ctx, client, endpoint)
	if err != nil {
		return nil, fmt.Errorf("fetch latest release metadata: %w", err)
	}

	var release releaseResponse
	if err := json.Unmarshal(payload, &release); err != nil {
		return nil, fmt.Errorf("decode latest release metadata: %w", err)
	}
	if release.TagName == "" {
		return nil, errors.New("latest release metadata missing tag_name")
	}
	if len(release.Assets) == 0 {
		return nil, fmt.Errorf("latest release %s has no assets", release.TagName)
	}

	return &release, nil
}

func fetchURL(ctx context.Context, client *http.Client, url string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", "sshelob-updater")

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer func(Body io.ReadCloser) {
		err := Body.Close()
		if err != nil {
			_, _ = fmt.Fprintf(os.Stderr, "warning: failed to close response body: %v\n", err)
		}
	}(resp.Body)

	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return nil, fmt.Errorf("%s: %s", resp.Status, strings.TrimSpace(string(body)))
	}

	return io.ReadAll(resp.Body)
}

func selectReleaseAsset(release *releaseResponse, goos, goarch string) (*releaseAsset, error) {
	ext := "tar.gz"
	if goos == "windows" {
		ext = "zip"
	}

	expectedName := fmt.Sprintf("sshelob_%s_%s_%s.%s", release.TagName, goos, goarch, ext)
	for i := range release.Assets {
		if release.Assets[i].Name == expectedName {
			return &release.Assets[i], nil
		}
	}

	return nil, fmt.Errorf("release %s does not contain asset %q", release.TagName, expectedName)
}

func extractBinary(archiveName, binaryName string, payload []byte) ([]byte, error) {
	if strings.HasSuffix(archiveName, ".zip") {
		return extractBinaryFromZip(binaryName, payload)
	}
	if strings.HasSuffix(archiveName, ".tar.gz") {
		return extractBinaryFromTarGz(binaryName, payload)
	}

	return nil, fmt.Errorf("unsupported archive format for asset %q", archiveName)
}

func extractBinaryFromZip(binaryName string, payload []byte) ([]byte, error) {
	r, err := zip.NewReader(bytes.NewReader(payload), int64(len(payload)))
	if err != nil {
		return nil, fmt.Errorf("open zip archive: %w", err)
	}

	for _, file := range r.File {
		if file.FileInfo().IsDir() {
			continue
		}
		if path.Base(file.Name) != binaryName {
			continue
		}

		reader, err := file.Open()
		if err != nil {
			return nil, fmt.Errorf("open binary %q in zip: %w", binaryName, err)
		}

		binary, err := io.ReadAll(reader)
		closeErr := reader.Close()
		if err != nil {
			return nil, fmt.Errorf("read binary %q from zip: %w", binaryName, err)
		}
		if closeErr != nil {
			return nil, fmt.Errorf("close binary %q in zip: %w", binaryName, closeErr)
		}

		return binary, nil
	}

	return nil, fmt.Errorf("binary %q not found in zip archive", binaryName)
}

func extractBinaryFromTarGz(binaryName string, payload []byte) ([]byte, error) {
	gzReader, err := gzip.NewReader(bytes.NewReader(payload))
	if err != nil {
		return nil, fmt.Errorf("open tar.gz archive: %w", err)
	}
	defer func(gzReader *gzip.Reader) {
		if closeErr := gzReader.Close(); closeErr != nil {
			err = closeErr
		}
	}(gzReader)

	tarReader := tar.NewReader(gzReader)
	for {
		header, err := tarReader.Next()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("read tar.gz archive entry: %w", err)
		}
		if header.Typeflag != tar.TypeReg {
			continue
		}
		if path.Base(header.Name) != binaryName {
			continue
		}

		binary, err := io.ReadAll(tarReader)
		if err != nil {
			return nil, fmt.Errorf("read binary %q from tar.gz: %w", binaryName, err)
		}

		return binary, nil
	}

	return nil, fmt.Errorf("binary %q not found in tar.gz archive", binaryName)
}

func replaceExecutable(executablePath string, binary []byte) (err error) {
	currentInfo, err := os.Stat(executablePath)
	if err != nil {
		return fmt.Errorf("stat current executable: %w", err)
	}

	tmpFile, err := os.CreateTemp(filepath.Dir(executablePath), ".sshelob-update-*")
	if err != nil {
		return fmt.Errorf("create temporary file for update: %w", err)
	}

	tmpPath := tmpFile.Name()
	defer func() {
		if removeErr := os.Remove(tmpPath); removeErr != nil && !errors.Is(removeErr, os.ErrNotExist) && err == nil {
			err = fmt.Errorf("cleanup temporary file: %w", removeErr)
		}
	}()

	if _, err := tmpFile.Write(binary); err != nil {
		_ = tmpFile.Close()
		return fmt.Errorf("write updated binary to temporary file: %w", err)
	}

	if err := tmpFile.Chmod(currentInfo.Mode() & os.ModePerm); err != nil {
		_ = tmpFile.Close()
		return fmt.Errorf("set permissions on temporary binary: %w", err)
	}

	if err := tmpFile.Close(); err != nil {
		return fmt.Errorf("close temporary binary file: %w", err)
	}

	if err := os.Rename(tmpPath, executablePath); err != nil {
		if runtime.GOOS == "windows" {
			if removeErr := os.Remove(executablePath); removeErr == nil {
				if renameErr := os.Rename(tmpPath, executablePath); renameErr == nil {
					return nil
				}
			}
		}

		return fmt.Errorf("replace executable in-place: %w", err)
	}

	return nil
}
