.PHONY: generate-models generate-server

# Variables
OAPI_CODEGEN := oapi-codegen
OPENAPI_FILE := api/openapi.yml
GEN_DIR := internal/infra/transport/rest/gen
GO_VERSION := 1.24.10

# generator:
# 	@go install github.com/oapi-codegen/oapi-codegen/v2/cmd/oapi-codegen@latest

# Generate code from OpenAPI spec
generate-models:
	@echo "Generating models from OpenAPI spec..."
	@mkdir -p $(GEN_DIR)
	$(OAPI_CODEGEN) -config configs/oapi/models.yaml $(OPENAPI_FILE)
	@echo "Code generation completed"

generate-server:
	@echo "Generating chi-server from OpenAPI spec..."
	@mkdir -p $(GEN_DIR)
	$(OAPI_CODEGEN) -config configs/oapi/server.yaml $(OPENAPI_FILE)
	@echo "Code generation completed"
