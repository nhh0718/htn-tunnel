package main

import (
	"archive/tar"
	"compress/gzip"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/nhh0718/htn-tunnel/internal/config"
	"github.com/spf13/cobra"
)

func backupCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "backup [output-path]",
		Short: "Backup config and API keys",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			outPath := "htn-tunnel-backup.tar.gz"
			if len(args) > 0 {
				outPath = args[0]
			}
			return runBackup(outPath)
		},
	}
}

func restoreCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "restore <backup-file>",
		Short: "Restore config and API keys from backup",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runRestore(args[0])
		},
	}
}

func runBackup(outPath string) error {
	fmt.Print("\n  htn-tunnel Backup\n\n")

	cfgPath := resolveConfigPath()
	cfg, _ := config.LoadServerConfig(cfgPath)

	var files []string
	if fileExists(cfgPath) {
		files = append(files, cfgPath)
	}
	if cfg != nil && fileExists(cfg.KeyStorePath) {
		files = append(files, cfg.KeyStorePath)
	}

	if len(files) == 0 {
		return fmt.Errorf("no files to backup (config: %s)", cfgPath)
	}

	if err := createTarGz(outPath, files); err != nil {
		return err
	}

	fmt.Printf("  Backed up %d file(s) to %s\n", len(files), outPath)
	for _, f := range files {
		fmt.Printf("    - %s\n", f)
	}
	fmt.Println()
	return nil
}

func runRestore(backupPath string) error {
	fmt.Print("\n  htn-tunnel Restore\n\n")
	fmt.Printf("  Extracting %s...\n", backupPath)

	restored, err := extractTarGz(backupPath)
	if err != nil {
		return err
	}

	for _, f := range restored {
		fmt.Printf("    + %s\n", f)
	}

	fmt.Print("\n  Restore complete. Restart the server to apply changes.\n")
	fmt.Print("    systemctl restart htn-tunnel\n\n")
	return nil
}

// resolveConfigPath returns the config path from the --config flag or the default.
func resolveConfigPath() string {
	if flagConfig != "" {
		return flagConfig
	}
	return "/etc/htn-tunnel/server.yaml"
}

// fileExists returns true when path exists and is a regular file.
func fileExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.Mode().IsRegular()
}

// createTarGz archives files (with their absolute paths preserved) into a .tar.gz.
func createTarGz(dest string, files []string) error {
	out, err := os.Create(dest)
	if err != nil {
		return fmt.Errorf("create archive: %w", err)
	}
	defer out.Close()

	gw := gzip.NewWriter(out)
	defer gw.Close()
	tw := tar.NewWriter(gw)
	defer tw.Close()

	for _, path := range files {
		if err := addFileToTar(tw, path); err != nil {
			return err
		}
	}
	return nil
}

func addFileToTar(tw *tar.Writer, path string) error {
	f, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("open %s: %w", path, err)
	}
	defer f.Close()

	info, err := f.Stat()
	if err != nil {
		return err
	}

	// Store with absolute path so restore can put files back in place.
	hdr := &tar.Header{
		Name:    path,
		Mode:    int64(info.Mode()),
		Size:    info.Size(),
		ModTime: info.ModTime(),
	}
	if err := tw.WriteHeader(hdr); err != nil {
		return fmt.Errorf("write tar header for %s: %w", path, err)
	}
	if _, err := io.Copy(tw, f); err != nil {
		return fmt.Errorf("write tar body for %s: %w", path, err)
	}
	return nil
}

// extractTarGz unpacks a .tar.gz archive, restoring files to their original paths.
// Returns the list of restored file paths.
func extractTarGz(src string) ([]string, error) {
	f, err := os.Open(src)
	if err != nil {
		return nil, fmt.Errorf("open archive: %w", err)
	}
	defer f.Close()

	gr, err := gzip.NewReader(f)
	if err != nil {
		return nil, fmt.Errorf("gzip reader: %w", err)
	}
	defer gr.Close()

	tr := tar.NewReader(gr)
	var restored []string

	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("read archive: %w", err)
		}
		if hdr.Typeflag != tar.TypeReg {
			continue
		}

		dest := hdr.Name
		if err := os.MkdirAll(filepath.Dir(dest), 0755); err != nil {
			return nil, fmt.Errorf("create dir for %s: %w", dest, err)
		}

		out, err := os.OpenFile(dest, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, os.FileMode(hdr.Mode))
		if err != nil {
			return nil, fmt.Errorf("create %s: %w", dest, err)
		}
		if _, err := io.Copy(out, tr); err != nil { //nolint:gosec
			out.Close()
			return nil, fmt.Errorf("write %s: %w", dest, err)
		}
		out.Close()
		restored = append(restored, dest)
	}

	if len(restored) == 0 {
		return nil, fmt.Errorf("archive contained no files")
	}
	return restored, nil
}
