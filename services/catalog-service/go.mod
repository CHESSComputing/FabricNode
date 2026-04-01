module github.com/CHESSComputing/FabricNode/services/catalog-service

go 1.26.1

require (
	github.com/CHESSComputing/FabricNode/pkg/config v0.0.0-20260401173051-e8aa9145f69b
	github.com/go-chi/chi/v5 v5.1.0
)

require github.com/CHESSComputing/FabricNode/pkg/server v0.0.0-20260401173051-e8aa9145f69b // indirect

require (
	github.com/CHESSComputing/FabricNode v0.0.0-20260330184757-be305e315636
	gopkg.in/yaml.v3 v3.0.1 // indirect
)

replace github.com/CHESSComputing/FabricNode/pkg/config => ../../pkg/config
