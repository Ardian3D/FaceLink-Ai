// Package enrollmentfixture composes migration files for disposable synthetic runtimes.
// It never opens a database, runs a migration, or reads production configuration.
package enrollmentfixture

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// PrepareMigrations copies the default migrations plus the isolated synthetic
// migrations into a fresh caller-selected directory. The normal migration runner
// remains responsible for validating filenames, ordering and checksums.
// destination must not exist; partial output on an I/O error is not reused.
func PrepareMigrations(baseDir, destination string) (string, error) {
	entries, err := os.ReadDir(baseDir)
	if err != nil {
		return "", fmt.Errorf("synthetic migrations: read base: %w", err)
	}
	isolatedDir := filepath.Join(filepath.Dir(filepath.Clean(baseDir)), "synthetic-migrations")
	isolated := []string{"00007_synthetic_enrollment.sql", "00008_template_aad_v2.sql"}
	for _, name := range isolated {
		info, statErr := os.Lstat(filepath.Join(isolatedDir, name))
		if statErr != nil || !info.Mode().IsRegular() {
			return "", errors.New("synthetic migrations: isolated migration unavailable")
		}
	}
	if err = os.Mkdir(destination, 0700); err != nil {
		return "", fmt.Errorf("synthetic migrations: destination must be fresh: %w", err)
	}
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".sql") {
			continue
		}
		if err = copyMigration(filepath.Join(baseDir, entry.Name()), filepath.Join(destination, entry.Name())); err != nil {
			return "", err
		}
	}
	for _, name := range isolated {
		if err = copyMigration(filepath.Join(isolatedDir, name), filepath.Join(destination, name)); err != nil {
			return "", err
		}
	}
	return destination, nil
}
func copyMigration(source, destination string) error {
	info, err := os.Lstat(source)
	if err != nil || !info.Mode().IsRegular() {
		return errors.New("synthetic migrations: source must be a regular file")
	}
	in, err := os.Open(source)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.OpenFile(destination, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if err != nil {
		return err
	}
	_, copyErr := io.Copy(out, in)
	syncErr := out.Sync()
	closeErr := out.Close()
	return errors.Join(copyErr, syncErr, closeErr)
}
