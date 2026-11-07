package main

import (
	"encoding/hex"
	"errors"
	"os"
	"path/filepath"

	"syntelle/render-worker/internal/nativemedia"
)

// Resolve one immutable package per admitted session. Assets are not cached
// globally: deletion and memory bounds remain effective for arbitrary users.
func privatePackages(root string, width, height int) (func(string) (nativemedia.SceneFactory, error), error) {
	if root == "" {
		return nil, nil
	}
	absolute, err := filepath.Abs(root)
	if err != nil {
		return nil, err
	}
	absolute, err = filepath.EvalSymlinks(absolute)
	if err != nil {
		return nil, errors.New("private package directory unavailable")
	}
	info, err := os.Stat(absolute)
	if err != nil || !info.IsDir() {
		return nil, errors.New("private package root must be a directory")
	}
	return func(hash string) (nativemedia.SceneFactory, error) {
		decoded, err := hex.DecodeString(hash)
		if err != nil || len(decoded) != 32 || hex.EncodeToString(decoded) != hash {
			return nil, errors.New("invalid private package identity")
		}
		path := filepath.Join(absolute, hash)
		real, err := filepath.EvalSymlinks(path)
		if err != nil {
			return nil, errors.New("private package is unavailable")
		}
		relative, err := filepath.Rel(absolute, real)
		if err != nil || relative != hash {
			return nil, errors.New("private package escapes registry")
		}
		factory, actual, err := loadSceneFactorySized(real, width, height)
		if err != nil {
			return nil, err
		}
		if actual != hash {
			return nil, errors.New("private package checksum mismatch")
		}
		return factory, nil
	}, nil
}
