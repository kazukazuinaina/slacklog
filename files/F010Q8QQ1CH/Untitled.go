package foo

type foo struct {
	Bar int
}

func NewFoo() *foo {
	return &foo{}
}

---

f := foo.NewFoo()
println(f.Bar)