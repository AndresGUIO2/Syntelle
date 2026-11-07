package main

import (
	"strings"
	"testing"
)

func TestPrivateRegistryRejectsPathsAndMissingPackages(t *testing.T) {
	resolve, err := privatePackages(t.TempDir(), 540, 720)
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"", "../assets", "/assets", strings.Repeat("A", 64), strings.Repeat("a", 64)} {
		t.Run(name, func(t *testing.T) {
			if _, err := resolve(name); err == nil {
				t.Fatal("unpublished package admitted")
			}
		})
	}
}
