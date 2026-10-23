//go:build !speech || !linux

package speech

import "syntelle/api/internal/conversation"

func NewAzureRecognition(string, string) conversation.Recognition { return nil }
