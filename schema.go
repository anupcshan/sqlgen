package main

import "time"

type Interface interface {
	IMethod()
}

//go:generate sqlgen -type=Foo
type Foo struct {
	// Primary key: id
	Id int64

	// Text: bar
	Bar string

	// Text: baz
	Baz string

	// Datetime: created
	Created time.Time

	// FK: type2
	Type2Ptr *Type2

	// FK: type3
	Type3Obj Type3
}

type Type2 struct {
}

type Type3 struct {
}
