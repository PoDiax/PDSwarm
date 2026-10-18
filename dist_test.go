package pdswarm_test

import (
	"io"
	"testing"

	"github.com/podiax/pdswarm"
)

func TestDistFS(t *testing.T) {
	wfs := pdswarm.WebFS()
	f, err := wfs.Open("index.html")
	if err != nil {
		t.Fatalf("failed to open index.html from WebFS: %v", err)
	}
	defer f.Close()

	content, err := io.ReadAll(f)
	if err != nil {
		t.Fatalf("failed to read index.html: %v", err)
	}

	if len(content) == 0 {
		t.Fatalf("index.html is empty")
	}

	for _, asset := range []string{"logo.svg", "favicon.svg", "icon.svg"} {
		af, err := wfs.Open(asset)
		if err != nil {
			t.Fatalf("failed to open %s from WebFS: %v", asset, err)
		}
		data, err := io.ReadAll(af)
		af.Close()
		if err != nil {
			t.Fatalf("failed to read %s: %v", asset, err)
		}
		if len(data) == 0 {
			t.Fatalf("%s is empty", asset)
		}
	}
}
