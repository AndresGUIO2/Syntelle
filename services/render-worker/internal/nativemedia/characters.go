package nativemedia

import (
	"encoding/hex"
	"errors"
	"syntelle/render-contract"
)

func (h *Host) resolveScene(settings rendercontract.Settings, hash string) (SceneFactory, string, error) {
	if settings.Character == rendercontract.Defaults().Character {
		if hash != "" && hash != h.config.PackageHash {
			return nil, "", errors.New("native character package mismatch")
		}
		return h.config.SceneFactory, h.config.PackageHash, nil
	}
	decoded, err := hex.DecodeString(hash)
	if !rendercontract.ValidCharacterID(settings.Character) || err != nil || len(decoded) != 32 || hex.EncodeToString(decoded) != hash || h.config.ResolvePackage == nil {
		return nil, "", errors.New("private character package unavailable")
	}
	factory, err := h.config.ResolvePackage(hash)
	if err != nil {
		return nil, "", err
	}
	if factory == nil {
		return nil, "", errors.New("private character scene unavailable")
	}
	return factory, hash, nil
}
