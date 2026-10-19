package v1alpha1

import "testing"

func TestPigzCompressionAlgorithm(t *testing.T) {
	if err := CompressPigz.Validate(); err != nil {
		t.Fatal(err)
	}
	extension, err := CompressPigz.Extension()
	if err != nil {
		t.Fatal(err)
	}
	if extension != "gz" {
		t.Fatalf("pigz extension = %q, want gz", extension)
	}
	algorithm, err := CompressionFromExtension(extension)
	if err != nil {
		t.Fatal(err)
	}
	if algorithm != CompressGzip {
		t.Fatalf("restore algorithm = %q, want gzip", algorithm)
	}
}
