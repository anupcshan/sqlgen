package sqlgen

import (
	"fmt"
	"go/ast"
	"go/build"
	"go/parser"
	"go/token"

	"github.com/golang/glog"
	"golang.org/x/tools/go/types"
)

type File struct {
	name       string
	parsedText *ast.File
}

type Parser struct {
	pkg   *types.Package
	dir   string
	files []*File
}

func NewParser() *Parser {
	return &Parser{files: []*File{}}
}

func (p *Parser) AddDirectory(directory string) {
	p.dir = directory
	glog.Infof("Adding directory: %s\n", directory)
	glog.Infof("Source directories: %s\n", build.Default.SrcDirs())

	pkg, err := build.ImportDir(directory, 0)
	if err != nil {
		glog.Fatalf("Error importing directory: %s\n", err)
	}

	// Currently, we don't include CgoFiles, TestGoFiles, SFiles etc...
	p.files = make([]*File, len(pkg.GoFiles))
	for i, name := range pkg.GoFiles {
		fName := fmt.Sprintf("%s/%s", directory, name)
		glog.Infof("Discovered file: %s\n", fName)
		p.files[i] = new(File)
		p.files[i].name = fName
	}
}

func (p *Parser) ParseFiles() {
	fs := token.NewFileSet()
	for _, file := range p.files {
		glog.Infof("Parsing file: %s\n", file.name)
		parsedFile, err := parser.ParseFile(fs, file.name, nil, 0)
		if err != nil {
			glog.Fatalf("Error parsing file: %s\n", err)
		}
		file.parsedText = parsedFile
		for _, imp := range parsedFile.Imports {
			glog.Infof("Import: %s\n", imp.Path.Value)
		}
	}
}
