package main

import (
	"flag"
	"os"

	"github.com/anupcshan/sqlgen/sqlgen"
)

var (
	typeNames = flag.String("type", "", "comma-separated list of type names [required]")
)

func main() {
	flag.Parse()

	if len(*typeNames) == 0 {
		flag.Usage()
		os.Exit(1)
	}

	args := flag.Args()
	if len(args) == 0 {
		args = []string{"."}
	}

	parser := sqlgen.NewParser()

	for _, dir := range args {
		parser.AddDirectory(dir)
	}

	parser.ParseFiles()
}
