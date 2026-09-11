// Package iagram exposes the embedded resource catalog shipped with the binary.
package iagram

import "embed"

// CatalogFS holds every catalog entry (YAML) and icon shipped in the binary.
//
//go:embed catalog/schema.json catalog/*/*.yaml catalog/icons/*/*.svg
var CatalogFS embed.FS
