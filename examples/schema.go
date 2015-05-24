package examples

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

	// Not supported: FKL type3
	Type3Obj Type3

	// One-to-many type4
	Type4List []*Type4
}

type Type2 struct {
	Id int64
}

type Type3 struct {
	Id int64
}

type Type4 struct {
	Id int64
}
