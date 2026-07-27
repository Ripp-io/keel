// Package archive creates tar.gz and zip archives from directories.
package archive

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

// CreateTarGz creates a .tar.gz archive of srcDir at destPath.
// Files are stored with paths relative to srcDir (no wrapper directory).
// Symlinks are skipped.
func CreateTarGz(srcDir, destPath string) (err error) {
	if err := os.MkdirAll(filepath.Dir(destPath), 0755); err != nil {
		return fmt.Errorf("creating parent directories for %q: %w", destPath, err)
	}

	outFile, err := os.Create(destPath)
	if err != nil {
		return fmt.Errorf("creating archive file %q: %w", destPath, err)
	}
	defer func() {
		if cerr := outFile.Close(); err == nil {
			err = cerr
		}
	}()

	gw := gzip.NewWriter(outFile)
	defer func() {
		if cerr := gw.Close(); err == nil {
			err = cerr
		}
	}()

	tw := tar.NewWriter(gw)
	defer func() {
		if cerr := tw.Close(); err == nil {
			err = cerr
		}
	}()

	walkErr := filepath.WalkDir(srcDir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}

		// Skip symlinks.
		if d.Type()&os.ModeSymlink != 0 {
			return nil
		}

		// Skip directories — they are implicit from file paths.
		if d.IsDir() {
			return nil
		}

		info, err := d.Info()
		if err != nil {
			return fmt.Errorf("stat %q: %w", path, err)
		}

		relPath, err := filepath.Rel(srcDir, path)
		if err != nil {
			return fmt.Errorf("computing relative path for %q: %w", path, err)
		}

		header := &tar.Header{
			Name:    filepath.ToSlash(relPath),
			Size:    info.Size(),
			Mode:    int64(info.Mode()),
			ModTime: info.ModTime(),
		}

		if err := tw.WriteHeader(header); err != nil {
			return fmt.Errorf("writing tar header for %q: %w", relPath, err)
		}

		f, err := os.Open(path)
		if err != nil {
			return fmt.Errorf("opening %q: %w", path, err)
		}
		defer func() { _ = f.Close() }()

		if _, err := io.Copy(tw, f); err != nil {
			return fmt.Errorf("writing %q to archive: %w", relPath, err)
		}

		return nil
	})
	if walkErr != nil {
		_ = os.Remove(destPath) // clean up partial archive
		return walkErr
	}
	return nil
}

// CreateZip creates a .zip archive of srcDir at destPath.
// Files are stored with paths relative to srcDir (no wrapper directory).
// Symlinks are skipped.
func CreateZip(srcDir, destPath string) (err error) {
	if err := os.MkdirAll(filepath.Dir(destPath), 0755); err != nil {
		return fmt.Errorf("creating parent directories for %q: %w", destPath, err)
	}

	outFile, err := os.Create(destPath)
	if err != nil {
		return fmt.Errorf("creating archive file %q: %w", destPath, err)
	}
	defer func() {
		if cerr := outFile.Close(); err == nil {
			err = cerr
		}
	}()

	zw := zip.NewWriter(outFile)
	defer func() {
		if cerr := zw.Close(); err == nil {
			err = cerr
		}
	}()

	walkErr := filepath.WalkDir(srcDir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}

		// Skip symlinks.
		if d.Type()&os.ModeSymlink != 0 {
			return nil
		}

		// Skip directories — they are implicit from file paths.
		if d.IsDir() {
			return nil
		}

		relPath, err := filepath.Rel(srcDir, path)
		if err != nil {
			return fmt.Errorf("computing relative path for %q: %w", path, err)
		}

		w, err := zw.Create(filepath.ToSlash(relPath))
		if err != nil {
			return fmt.Errorf("creating zip entry for %q: %w", relPath, err)
		}

		f, err := os.Open(path)
		if err != nil {
			return fmt.Errorf("opening %q: %w", path, err)
		}
		defer func() { _ = f.Close() }()

		if _, err := io.Copy(w, f); err != nil {
			return fmt.Errorf("writing %q to archive: %w", relPath, err)
		}

		return nil
	})
	if walkErr != nil {
		_ = os.Remove(destPath) // clean up partial archive
		return walkErr
	}
	return nil
}
