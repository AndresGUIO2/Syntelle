package nativerender

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"testing"
)

func TestReadCheckedRejectsChangedOrEscapingResources(t *testing.T) {
	root := t.TempDir()
	data := []byte{1, 2, 3, 4}
	sum := sha256.Sum256(data)
	hash := hex.EncodeToString(sum[:])
	if err := os.WriteFile(filepath.Join(root, "texture.rgba"), data, 0600); err != nil {
		t.Fatal(err)
	}
	actual, err := readChecked(root, "texture.rgba", hash)
	if err != nil || string(actual) != string(data) {
		t.Fatal("valid immutable resource did not load", err)
	}
	for _, name := range []string{"../texture.rgba", filepath.Join(root, "texture.rgba"), "..\\texture.rgba"} {
		if _, err := readChecked(root, name, hash); err == nil {
			t.Fatalf("escaping resource accepted: %q", name)
		}
	}
	if err := os.WriteFile(filepath.Join(root, "texture.rgba"), []byte{1, 2, 3, 5}, 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := readChecked(root, "texture.rgba", hash); err == nil {
		t.Fatal("changed source bytes accepted")
	}
}
