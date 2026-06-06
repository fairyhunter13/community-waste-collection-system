// Package apispec embeds the OpenAPI 3.0 specification for use at runtime.
package apispec

import _ "embed"

// Spec holds the embedded OpenAPI 3.0 YAML specification.
//
//go:embed openapi.yaml
var Spec []byte
