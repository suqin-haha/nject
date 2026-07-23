package demo

import nject "github.com/muir/nject/v2"

func ProvideName() string {
	return "nject"
}

func HelloNject(name string) string {
	return "Hello, " + name
}

func RunProviders(providers ...any) {
	nject.MustRun("LSP demo", providers...)
}

func Start() {
	RunProviders(ProvideName, HelloNject)
}
