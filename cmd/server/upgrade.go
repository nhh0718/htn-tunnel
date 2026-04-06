package main

import (
	"archive/tar"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"runtime"
	"strings"

	"github.com/spf13/cobra"
)

func upgradeCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "upgrade",
		Short: "Upgrade to the latest version",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runUpgrade()
		},
	}
}

func runUpgrade() error {
	fmt.Print("\n  htn-tunnel Server Upgrade\n\n")
	fmt.Printf("  Current version: %s\n", version)

	fmt.Printf("  Checking latest version...")
	latest, downloadURL, err := getLatestRelease()
	if err != nil {
		return fmt.Errorf("check update: %w", err)
	}
	fmt.Printf(" %s\n", latest)

	if latest == version {
		fmt.Println("  Already up to date!")
		return nil
	}

	fmt.Printf("  Downloading %s...", latest)
	tmpPath, err := downloadBinary(downloadURL)
	if err != nil {
		return fmt.Errorf("download: %w", err)
	}
	defer os.Remove(tmpPath)
	fmt.Println(" OK")

	fmt.Printf("  Replacing binary...")
	execPath, err := os.Executable()
	if err != nil {
		return fmt.Errorf("resolve executable: %w", err)
	}
	if err := replaceBinary(tmpPath, execPath); err != nil {
		return fmt.Errorf("replace: %w", err)
	}
	fmt.Println(" OK")

	fmt.Printf("  Restarting service...")
	exec.Command("systemctl", "restart", "htn-tunnel").Run() //nolint:errcheck
	fmt.Println(" OK")

	fmt.Printf("\n  Upgraded to %s!\n\n", latest)
	return nil
}

// githubRelease holds the fields we need from the GitHub releases API.
type githubRelease struct {
	TagName string `json:"tag_name"`
	Assets  []struct {
		Name               string `json:"name"`
		BrowserDownloadURL string `json:"browser_download_url"`
	} `json:"assets"`
}

func getLatestRelease() (ver, url string, err error) {
	resp, err := http.Get("https://api.github.com/repos/nhh0718/htn-tunnel/releases/latest") //nolint:noctx
	if err != nil {
		return "", "", err
	}
	defer resp.Body.Close()

	var rel githubRelease
	if err := json.NewDecoder(resp.Body).Decode(&rel); err != nil {
		return "", "", fmt.Errorf("parse release: %w", err)
	}

	tag := strings.TrimPrefix(rel.TagName, "v")
	// Find asset matching: htn-server_{version}_{os}_{arch}.tar.gz
	want := fmt.Sprintf("htn-server_%s_%s_%s.tar.gz", tag, runtime.GOOS, runtime.GOARCH)
	for _, a := range rel.Assets {
		if a.Name == want {
			return tag, a.BrowserDownloadURL, nil
		}
	}
	return "", "", fmt.Errorf("no asset found for %s/%s (wanted %s)", runtime.GOOS, runtime.GOARCH, want)
}

// downloadBinary fetches a .tar.gz from url, extracts the htn-server binary,
// and returns the path to a temp file containing it.
func downloadBinary(url string) (string, error) {
	resp, err := http.Get(url) //nolint:noctx
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	gz, err := gzip.NewReader(resp.Body)
	if err != nil {
		return "", fmt.Errorf("gzip reader: %w", err)
	}
	defer gz.Close()

	tr := tar.NewReader(gz)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return "", fmt.Errorf("tar read: %w", err)
		}
		// Accept any entry named "htn-server" (no path prefix required).
		if hdr.Typeflag != tar.TypeReg || !strings.HasSuffix(hdr.Name, "htn-server") {
			continue
		}

		tmp, err := os.CreateTemp("", "htn-server-*")
		if err != nil {
			return "", err
		}
		if _, err := io.Copy(tmp, tr); err != nil { //nolint:gosec
			tmp.Close()
			os.Remove(tmp.Name())
			return "", fmt.Errorf("extract binary: %w", err)
		}
		tmp.Close()
		return tmp.Name(), nil
	}
	return "", fmt.Errorf("htn-server binary not found in archive")
}

// replaceBinary swaps newPath into currentPath; backs up the old binary first.
func replaceBinary(newPath, currentPath string) error {
	backupPath := currentPath + ".bak"
	// Move current binary to .bak (best-effort).
	os.Rename(currentPath, backupPath) //nolint:errcheck

	src, err := os.Open(newPath)
	if err != nil {
		return err
	}
	defer src.Close()

	dst, err := os.OpenFile(currentPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0755)
	if err != nil {
		// Restore backup if we can't create the new file.
		os.Rename(backupPath, currentPath) //nolint:errcheck
		return err
	}
	defer dst.Close()

	if _, err := io.Copy(dst, src); err != nil {
		os.Rename(backupPath, currentPath) //nolint:errcheck
		return fmt.Errorf("copy binary: %w", err)
	}
	return nil
}
