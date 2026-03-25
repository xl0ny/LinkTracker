package botopenapi

import _ "embed"

// ContractYAML — OpenAPI 3.1 спецификация Bot API (для Swagger UI).
//
//go:embed contract.yaml
var ContractYAML []byte
