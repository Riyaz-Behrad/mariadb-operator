package compression

import (
	"bytes"
	"context"
	"io"
	"os/exec"
	"testing"

	mariadbv1alpha1 "github.com/mariadb-operator/mariadb-operator/v26/api/v1alpha1"
)

func TestPigzGzipCompatibility(t *testing.T) {
	if _, err := exec.LookPath("pigz"); err != nil {
		t.Skip("pigz is not installed")
	}
	compressor, err := NewCompressor(mariadbv1alpha1.CompressPigz)
	if err != nil {
		t.Fatal(err)
	}
	for _, content := range [][]byte{nil, bytes.Repeat([]byte("MariaDB backup data\n"), 100000)} {
		var compressed, restored bytes.Buffer
		if err := compressor.Compress(context.Background(), &compressed, bytes.NewReader(content)); err != nil {
			t.Fatal(err)
		}
		if err := (&GzipCompressor{}).Decompress(context.Background(), &restored, &compressed); err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(restored.Bytes(), content) {
			t.Fatal("gzip decompression did not recover the original data")
		}
	}
}

func TestPigzCompressionCanceled(t *testing.T) {
	if _, err := exec.LookPath("pigz"); err != nil {
		t.Skip("pigz is not installed")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := (&PigzCompressor{}).Compress(ctx, io.Discard, bytes.NewReader(nil)); err == nil {
		t.Fatal("expected canceled compression to fail")
	}
}
