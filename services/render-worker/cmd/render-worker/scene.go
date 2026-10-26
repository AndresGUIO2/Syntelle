package main

import (
	"syntelle/render-worker/internal/nativeadapter"
	"syntelle/render-worker/internal/nativemedia"
)

func loadSceneFactory(directory string) (nativemedia.SceneFactory, string, error) {
	return nativeadapter.LoadForeground(directory)
}

func loadSceneFactorySized(directory string, width, height int) (nativemedia.SceneFactory, string, error) {
	return nativeadapter.LoadForegroundSized(directory, width, height)
}
