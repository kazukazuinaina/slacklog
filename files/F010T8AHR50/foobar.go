package main

type Fooer interface {
	Foo()
}

type Barer interface {
	Bar()
}

func callFooOrBar(v interface{}) {
	switch w := v.(type) {
	case Fooer:
		w.Foo()
	case Barer:
		w.Bar()
	}
}

type foo struct{}

func (f *foo) Foo() {
	println("foo.Foo() called")
}

type bar struct{}

func (b *bar) Bar() {
	println("bar.Bar() called")
}

type foobar struct{}

func (fb *foobar) Foo() {
	println("foobar.Foo() called")
}

func (fb *foobar) Bar() {
	println("foobar.Bar() called")
}

func main() {
	f := &foo{}
	callFooOrBar(f)

	b := &bar{}
	callFooOrBar(b)

	fb := &foobar{}
	callFooOrBar(fb)
}
