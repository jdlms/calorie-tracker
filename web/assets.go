// Package web embeds static frontend assets.
package web

import "embed"

// Assets contains embedded frontend files.
//
//go:embed *
var Assets embed.FS
