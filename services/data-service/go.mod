module github.com/CHESSComputing/FabricNode/services/data-service

go 1.26.1

require (
	github.com/go-chi/chi/v5 v5.1.0
	github.com/google/uuid v1.6.0
)

require github.com/CHESSComputing/FabricNode v0.0.0-20260327135358-b47faf3157a3 // indirect

replace github.com/CHESSComputing/FabricNode => ../../
