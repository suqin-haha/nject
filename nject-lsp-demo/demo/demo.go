package demo

func HelloNject(name string) string {
	return "Hello, " + name
}

func (g Greeter) Greet(name string) string {
	return HelloNject(name)
}

type Greeter struct{}
