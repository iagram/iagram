// Package iagram exposes the embedded resource catalog shipped with the binary.
package iagram

import "embed"

// CatalogFS holds every catalog entry (YAML) and icon shipped in the binary.
//
//go:embed catalog/schema.json catalog/equivalences.yaml catalog/*/*.yaml catalog/icons/*/*.svg catalog/icons/*/services.yaml catalog/icons/*/services/*.svg catalog/icons/*/resources/*.svg catalog/icons/*/groups/*.svg
var CatalogFS embed.FS

// ModulesFS holds the Terraform modules referenced by the catalog.
//
//go:embed modules/*/*/*.tf
var ModulesFS embed.FS

// SchemasFS holds compact, gzipped provider schema snapshots produced by
// `iagram schemas update --write schemas/`; they let iagram offer an element
// for every resource type a provider has, offline.
//
//go:embed schemas/*.json.gz
var SchemasFS embed.FS

// TemplatesFS holds the reference architectures (templates/<provider>/<slug>/
// template.yaml) listed by the gallery and opened pre-configured.
//
//go:embed templates/*/*/template.yaml templates/*/*/diagram.*
var TemplatesFS embed.FS
