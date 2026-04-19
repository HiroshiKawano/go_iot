// Package docs は Scalar API ドキュメントの埋め込みアセットを提供する。
// OpenAPI 仕様 (openapi.yaml) と Scalar UI (index.html) をバイナリに同梱し、
// main 側から Echo のレスポンスとして返却する。
package docs

import _ "embed"

//go:embed openapi.yaml
var OpenAPIYAML []byte

//go:embed index.html
var IndexHTML []byte
