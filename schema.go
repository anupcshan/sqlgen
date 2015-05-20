package main

type Interface interface {
	IMethod()
}

//go:generate sqlgen -type=Foo
type Foo struct {
	// Primary key: id
	Id int64

	// Text: bar
	Bar string
}
