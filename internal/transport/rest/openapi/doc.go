package openapi

import _ "embed"

// SwaggerJSON contains the embedded OpenAPI 2.0 specification in JSON format.
//
//go:embed swagger.json
var SwaggerJSON []byte

// SwaggerYAML contains the embedded OpenAPI 2.0 specification in YAML format.
//
//go:embed swagger.yaml
var SwaggerYAML []byte
