// Package web embeds static frontend assets.
package web

import "embed"

// Assets contains embedded frontend files.
//
//go:embed assets.go dist/*
var Assets embed.FS
