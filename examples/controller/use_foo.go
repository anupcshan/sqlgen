package examples

import "fmt"
import "github.com/anupcshan/sqlgen/examples/model"

func CreateNewFoo(f *model.FooQuery) {
	if tx, err := f.Transaction(); err != nil {
		fmt.Printf("Error creating transaction: %s\n", err)
	} else {
		cf, ce := tx.ByBar("bar")
		select {
		case foo := <-cf:
			fmt.Printf("Found foo: %s\n", foo)
		case err := <-ce:
			fmt.Printf("Found error: %s\n", err)
			break
		}
	}
}
