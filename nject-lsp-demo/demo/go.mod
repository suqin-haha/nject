module example.com/nject-lsp-demo

go 1.22

require github.com/muir/nject/v2 v2.0.0

require (
	github.com/muir/reflectutils v0.11.0 // indirect
	github.com/pkg/errors v0.9.1 // indirect
)

replace github.com/muir/nject/v2 => ../..
