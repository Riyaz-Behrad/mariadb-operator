package compression

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/go-logr/logr"
	mariadbv1alpha1 "github.com/mariadb-operator/mariadb-operator/v26/api/v1alpha1"
	"github.com/mariadb-operator/mariadb-operator/v26/pkg/backup"
)

func TestBackupCompressors(t *testing.T) {
	content := "Lorem ipsum dolor sit amet, consectetur adipiscing elit."
	processor := backup.NewLogicalBackupProcessor()
	logger := logr.Discard()

	tests := []struct {
		name            string
		newCompressorFn func(basePath string, getUncompressedFilename GetBackupUncompressedFilenameFn, logger logr.Logger) BackupCompressor
		fileName        string
	}{
		{
			name:            "nop",
			newCompressorFn: NewNopBackupCompressor,
			fileName:        "backup.2023-12-18T16:14:00Z.sql",
		},
		{
			name:            "gzip",
			newCompressorFn: NewGzipBackupCompressor,
			fileName:        "backup.2023-12-18T16:14:00Z.sql.gz",
		},
		{
			name:            "pigz",
			newCompressorFn: NewPigzBackupCompressor,
			fileName:        "backup.2023-12-18T16:14:00Z.sql.gz",
		},
		{
			name:            "bzip2",
			newCompressorFn: NewBzip2BackupCompressor,
			fileName:        "backup.2023-12-18T16:14:00Z.sql.bz2",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.name == "pigz" {
				if _, err := exec.LookPath("pigz"); err != nil {
					t.Skip("pigz is not installed")
				}
			}
			dir, err := os.MkdirTemp("", "backup_test")
			if err != nil {
				t.Fatalf("Failed to create temp dir: %v", err)
			}
			defer os.RemoveAll(dir)

			compressor := tt.newCompressorFn(dir, processor.GetUncompressedBackupFile, logger)

			filePath := filepath.Join(dir, tt.fileName)
			if err := os.WriteFile(filePath, []byte(content), 0644); err != nil {
				t.Fatalf("Failed to write test file: %v", err)
			}

			if err := compressor.Compress(filePath); err != nil {
				t.Fatalf("Failed to compress test file: %v", err)
			}
			decompressedFileName, err := compressor.Decompress(filePath)
			if err != nil {
				t.Fatalf("Failed to decompress test file: %v", err)
			}

			decompressedContent, err := os.ReadFile(decompressedFileName)
			if err != nil {
				t.Fatalf("Failed to read decompressed file: %v", err)
			}
			if string(decompressedContent) != content {
				t.Errorf("Decompressed content does not match original content:\nGot: %s\nWant: %s", decompressedContent, content)
			}
		})
	}
}

func TestPigzBackupRestore(t *testing.T) {
	if _, err := exec.LookPath("pigz"); err != nil {
		t.Skip("pigz is not installed")
	}
	tests := []struct {
		name      string
		fileName  string
		processor backup.BackupProcessor
	}{
		{"logical", "backup.2023-12-18T16:14:00Z.sql.gz", backup.NewLogicalBackupProcessor()},
		{"physical", "physicalbackup-20231218161400.xb.gz", backup.NewPhysicalBackupProcessor()},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			content := []byte("backup data")
			filePath := filepath.Join(dir, tt.fileName)
			if err := os.WriteFile(filePath, content, 0600); err != nil {
				t.Fatal(err)
			}
			compressor, err := NewBackupCompressor(mariadbv1alpha1.CompressPigz, dir,
				tt.processor.GetUncompressedBackupFile, logr.Discard())
			if err != nil {
				t.Fatal(err)
			}
			if err := compressor.Compress(tt.fileName); err != nil {
				t.Fatal(err)
			}
			// Restore selects the decompressor from the filename, without the Backup CR.
			algorithm, err := tt.processor.ParseCompressionAlgorithm(tt.fileName)
			if err != nil {
				t.Fatal(err)
			}
			restoreCompressor, err := NewBackupCompressor(algorithm, dir,
				tt.processor.GetUncompressedBackupFile, logr.Discard())
			if err != nil {
				t.Fatal(err)
			}
			plainPath, err := restoreCompressor.Decompress(tt.fileName)
			if err != nil {
				t.Fatal(err)
			}
			got, err := os.ReadFile(plainPath)
			if err != nil {
				t.Fatal(err)
			}
			if string(got) != string(content) {
				t.Fatalf("restored content = %q, want %q", got, content)
			}
		})
	}
}

func TestPigzBackupCompressionFailure(t *testing.T) {
	// A failed compressor must preserve the source backup and remove its temporary output.
	t.Setenv("PATH", t.TempDir())
	dir := t.TempDir()
	fileName := "backup.2023-12-18T16:14:00Z.sql.gz"
	filePath := filepath.Join(dir, fileName)
	content := []byte("backup data")
	if err := os.WriteFile(filePath, content, 0600); err != nil {
		t.Fatal(err)
	}
	compressor := NewPigzBackupCompressor(dir, backup.NewLogicalBackupProcessor().GetUncompressedBackupFile, logr.Discard())
	if err := compressor.Compress(fileName); err == nil {
		t.Fatal("expected missing pigz to fail compression")
	}
	got, err := os.ReadFile(filePath)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != string(content) {
		t.Fatalf("source backup changed: got %q, want %q", got, content)
	}
	if _, err := os.Stat(filePath + ".tmp"); !os.IsNotExist(err) {
		t.Fatalf("temporary output was not removed: %v", err)
	}
}
