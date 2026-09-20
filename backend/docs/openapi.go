package docs

import _ "embed"

// OpenAPI contains the versioned HTTP contract served by the API.
//
//go:embed openapi.yaml
var OpenAPI []byte
