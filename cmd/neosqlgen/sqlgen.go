package main

import (
	"flag"

	"github.com/anupcshan/sqlgen/sqlgen"
	"github.com/golang/glog"
)

func main() {
	flag.Parse()

	args := flag.Args()
	if len(args) == 0 {
		args = []string{"."}
	}

	parser := sqlgen.NewParser()

	for _, dir := range args {
		if err := parser.AddDirectory(dir); err != nil {
			glog.Fatalf("Error adding directory: %s\n", err)
		}
	}

	parser.ParseFiles()
}
