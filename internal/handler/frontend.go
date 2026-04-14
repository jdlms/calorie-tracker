package handler

import (
	"fmt"
	"io/fs"
	"net/http"
)

// NewFrontendHandler serves embedded static frontend assets.
func NewFrontendHandler(assets fs.FS) (http.Handler, error) {
	sub, err := fs.Sub(assets, "dist")
	if err != nil {
		return nil, fmt.Errorf("creating frontend sub filesystem: %w", err)
	}

	return http.FileServer(http.FS(sub)), nil
}
