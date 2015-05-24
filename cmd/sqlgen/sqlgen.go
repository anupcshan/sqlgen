package main

import (
	"bytes"
	"flag"
	"fmt"
	"go/ast"
	"go/build"
	"go/format"
	"go/parser"
	"go/token"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"strings"

	"golang.org/x/tools/go/types"

	_ "golang.org/x/tools/go/gcimporter"
)

var (
	typeNames = flag.String("type", "", "comma-separated list of type names; must be set")
	output    = flag.String("output", "", "output file name; default srcdir/<type>_query.go")
)

// Usage is a replacement usage function for the flags package.
func Usage() {
	fmt.Fprintf(os.Stderr, "Usage of %s:\n", os.Args[0])
	fmt.Fprintf(os.Stderr, "\tsqlgen [flags] -type T [directory]\n")
	fmt.Fprintf(os.Stderr, "\tsqlgen [flags[ -type T files... # Must be a single package\n")
	fmt.Fprintf(os.Stderr, "Flags:\n")
	flag.PrintDefaults()
}

func main() {
	log.SetFlags(0)
	log.SetPrefix("sqlgen: ")
	flag.Usage = Usage
	flag.Parse()
	if len(*typeNames) == 0 {
		flag.Usage()
		os.Exit(2)
	}
	types := strings.Split(*typeNames, ",")

	// We accept either one directory or a list of files. Which do we have?
	args := flag.Args()
	if len(args) == 0 {
		// Default: process whole package in current directory.
		args = []string{"."}
	}

	var (
		dir string
		g   Generator
	)
	if len(args) == 1 && isDirectory(args[0]) {
		dir = args[0]
		g.parsePackageDir(args[0])
	} else {
		dir = filepath.Dir(args[0])
		g.parsePackageFiles(args)
	}

	// Print the header and package clause.
	g.Printf("// generated by sqlgen %s; DO NOT EDIT\n", strings.Join(os.Args[1:], " "))
	g.Printf("\n")
	g.Printf("package %s", g.pkg.name)
	g.Printf("\n")
	g.Printf("import \"database/sql\"\n")

	// Run generate for each type.
	for _, typeName := range types {
		g.generate(typeName)
	}

	// Format the output.
	src := g.format()

	// Write to file.
	outputName := *output
	if outputName == "" {
		baseName := fmt.Sprintf("%s_query.go", types[0])
		outputName = filepath.Join(dir, strings.ToLower(baseName))
	}
	err := ioutil.WriteFile(outputName, src, 0644)
	if err != nil {
		log.Fatalf("writing output: %s", err)
	}
}

// Generator holds the state of the analysis. Primarily used to buffer
// the output for format.Source.
type Generator struct {
	buf               bytes.Buffer // Accumulated output.
	pkg               *Package     // Package we are scanning.
	additionalImports []string
}

type Package struct {
	dir      string
	name     string
	defs     map[*ast.Ident]types.Object
	files    []*File
	typesPkg *types.Package
}

// File holds a single parsed file and associated data.
type File struct {
	pkg  *Package  // Package to which this file belongs.
	file *ast.File // Parsed AST.

	// Following fields are reset for each type being generated.
	typeName          string  // Name of the struct type.
	fields            []Field // Accumulator for fields of that type.
	additionalImports []string
}

// Value represents a declared field.
type Field struct {
	srcName string // Field name in source
	dbName  string // Field name in DB
	isPK    bool   // Is the field a primary key?
	srcType string // Field type in source
	dbType  string // Expected field type in the DB
}

// isDirectory reports whether the named file is a directory.
func isDirectory(name string) bool {
	info, err := os.Stat(name)
	if err != nil {
		log.Fatal(err)
	}
	return info.IsDir()
}

// parsePackageDir parses the package residing in the directory.
func (g *Generator) parsePackageDir(directory string) {
	pkg, err := build.Default.ImportDir(directory, 0)
	if err != nil {
		log.Fatalf("cannot process directory %s: %s", directory, err)
	}
	var names []string
	names = append(names, pkg.GoFiles...)
	names = append(names, pkg.CgoFiles...)
	// TODO: Need to think about constants in test files. Maybe write type_query_test.go
	// in a separate pass? For later. names = append(names, pkg.TestGoFiles...)
	// These are also in the "foo" package.
	names = append(names, pkg.SFiles...)
	names = prefixDirectory(directory, names)
	g.parsePackage(directory, names, nil)
}

func (g *Generator) Printf(format string, args ...interface{}) {
	fmt.Fprintf(&g.buf, format, args...)
}

// parsePackageFiles parses the package occupying the named files.
func (g *Generator) parsePackageFiles(names []string) {
	g.parsePackage(".", names, nil)
}

// parsePackage analyzes the single package constructed from the named files.
// If text is non-nil, it is a string to be used instead of the content of the file,
// to be used for testing. parsePackage exits if there is an error.
func (g *Generator) parsePackage(directory string, names []string, text interface{}) {
	var files []*File
	var astFiles []*ast.File
	g.pkg = new(Package)
	fs := token.NewFileSet()
	for _, name := range names {
		if !strings.HasSuffix(name, ".go") {
			continue
		}
		parsedFile, err := parser.ParseFile(fs, name, text, 0)
		if err != nil {
			log.Fatalf("parsing package: %s: %s", name, err)
		}
		astFiles = append(astFiles, parsedFile)
		files = append(files, &File{
			file: parsedFile,
			pkg:  g.pkg,
		})
	}
	if len(astFiles) == 0 {
		log.Fatalf("%s: no buildable Go files", directory)
	}
	g.pkg.name = astFiles[0].Name.Name
	g.pkg.files = files
	g.pkg.dir = directory
	// Type check the package.
	g.pkg.check(fs, astFiles)
}

func (g *Generator) format() []byte {
	src, err := format.Source(g.buf.Bytes())
	if err != nil {
		// Should never happen, but can arise when developing this code.
		// The user can compile the output to see the error.
		log.Printf("warning: internal error: invalid Go generated: %s", err)
		log.Printf("warning: compile the package to analyze the error")
		return g.buf.Bytes()
	}
	return src
}

// generate produces the String method for the named type.
func (g *Generator) generate(typeName string) {
	fields := make([]Field, 0, 100)
	for _, file := range g.pkg.files {
		// Set the state for this run of the walker.
		file.typeName = typeName
		file.fields = nil
		if file.file != nil {
			ast.Inspect(file.file, file.genDecl)
			g.additionalImports = append(g.additionalImports, file.additionalImports...)
			fields = append(fields, file.fields...)
		}
	}

	if len(fields) == 0 {
		log.Fatalf("no values defined for type %s", typeName)
	}

	g.build(fields, typeName)
}

//go:generate stringer -type=SourceType
type SourceType int

const (
	ST_UNKNOWN SourceType = iota
	ST_INT64
	ST_INT
	ST_STRING
	ST_TIME
)

var KNOWN_SOURCE_TYPES = map[string]SourceType{
	"int64":     ST_INT64,
	"int":       ST_INT,
	"string":    ST_STRING,
	"time.Time": ST_TIME,
}

type KnownDBType string

const (
	DB_INTEGER   KnownDBType = "INTEGER"
	DB_BIGINT    KnownDBType = "BIGINT"
	DB_VARCHAR   KnownDBType = "VARCHAR"
	DB_TIMESTAMP KnownDBType = "TIMESTAMP"
)

type GenericType int

//go:generate stringer -type=GenericType
const (
	GT_NUMERIC GenericType = iota
	GT_STRING
	GT_TIMESTAMP
)

var SRCTYPE_TO_GENERICTYPE_MAP = map[SourceType]GenericType{
	ST_INT64:  GT_NUMERIC,
	ST_INT:    GT_NUMERIC,
	ST_STRING: GT_STRING,
	ST_TIME:   GT_TIMESTAMP,
}

var GENERICTYPE_TO_DBTYPE_MAP = map[GenericType][]KnownDBType{
	GT_NUMERIC:   []KnownDBType{DB_INTEGER, DB_BIGINT},
	GT_STRING:    []KnownDBType{DB_VARCHAR},
	GT_TIMESTAMP: []KnownDBType{DB_TIMESTAMP},
}

func srcTypeToFirstDbType(srcType SourceType) KnownDBType {
	genericType := SRCTYPE_TO_GENERICTYPE_MAP[srcType]
	return GENERICTYPE_TO_DBTYPE_MAP[genericType][0]
}

// genDecl processes one declaration clause.
func (f *File) genDecl(node ast.Node) bool {
	decl, ok := node.(*ast.GenDecl)

	if !ok || decl.Tok != token.TYPE {
		// We only care about types declarations.
		return true
	}

	// Loop over the elements of the declaration. Each element is a ValueSpec:
	// a list of names possibly followed by a type, possibly followed by values.
	// If the type and value are both missing, we carry down the type (and value,
	// but the "go/types" package takes care of that).
	for _, spec := range decl.Specs {
		tspec := spec.(*ast.TypeSpec) // Guaranteed to succeed as this is TYPE.

		if tspec.Name.Name != f.typeName {
			// Not the type we're looking for.
			continue
		}

		log.Printf("Type spec: %v name: %s\n", tspec.Type, tspec.Name.Name)

		if structType, ok := tspec.Type.(*ast.StructType); ok {
			log.Printf("Located the struct type: %v\n", structType)

			for _, field := range structType.Fields.List {
				log.Printf("Field: %v\n", field)

				if ident, ok := field.Type.(*ast.Ident); ok {
					// Look at list of known types and determine if we have a translation.
					tp := KNOWN_SOURCE_TYPES[ident.Name]

					if tp != ST_UNKNOWN {
						log.Printf("Primitive or local type found: %v => %s\n", ident.Name, tp.String())
					} else {
						// TODO: We should probably consider all of these fields as local objects and add
						// foreign key links.
						log.Printf("UNRECOGNIZED LOCAL TYPE seen: %v\n", ident.Name)
						continue
					}

					if len(field.Names) == 1 {
						fieldName := field.Names[0].Name
						isPK := false

						if strings.ToLower(fieldName) == "id" {
							isPK = true
						}

						f.fields = append(f.fields,
							Field{
								srcName: fieldName,
								dbName:  strings.ToLower(fieldName), // TODO: Override with annotations
								isPK:    isPK,
								srcType: ident.Name,
								dbType:  "string",
							})
					}
				} else if selector, ok := field.Type.(*ast.SelectorExpr); ok {
					// TODO: This likely means an object in another package. Foreign link?
					log.Printf("Found selector: %s :: %s\n", selector.X, selector.Sel.Name)
					typeName := fmt.Sprintf("%s.%s", selector.X, selector.Sel.Name)

					tp := KNOWN_SOURCE_TYPES[typeName]

					if tp != ST_UNKNOWN {
						log.Printf("Primitive or local type found: %v => %s\n", typeName, tp.String())
						f.additionalImports = append(f.additionalImports, fmt.Sprintf("%s", selector.X))
					} else {
						// TODO: We should probably consider all of these fields as local objects and add
						// foreign key links.
						log.Printf("UNRECOGNIZED LOCAL TYPE seen: %v\n", typeName)
						continue
					}

					if len(field.Names) == 1 {
						fieldName := field.Names[0].Name
						isPK := false

						if strings.ToLower(fieldName) == "id" {
							isPK = true
						}

						f.fields = append(f.fields,
							Field{
								srcName: fieldName,
								dbName:  strings.ToLower(fieldName), // TODO: Override with annotations
								isPK:    isPK,
								srcType: typeName,
								dbType:  "string",
							})
					}
				} else {
					// TODO: Enumerate all different possible types here.
					log.Printf("UNKNOWN TYPE seen: %v\n", field.Type)
				}
			}
		}
	}
	return false
}

// check type-checks the package. The package must be OK to proceed.
func (pkg *Package) check(fs *token.FileSet, astFiles []*ast.File) {
	pkg.defs = make(map[*ast.Ident]types.Object)
	config := types.Config{FakeImportC: true}
	info := &types.Info{
		Defs: pkg.defs,
	}
	typesPkg, err := config.Check(pkg.dir, fs, astFiles, info)
	if err != nil {
		log.Fatalf("checking package: %s", err)
	}
	pkg.typesPkg = typesPkg
}

// prefixDirectory places the directory name on the beginning of each name in the list.
func prefixDirectory(directory string, names []string) []string {
	if directory == "." {
		return names
	}
	ret := make([]string, len(names))
	for i, name := range names {
		ret[i] = filepath.Join(directory, name)
	}
	return ret
}

const newQueryDefn = `func New%[1]s(db *sql.DB) (*%[1]s, error) {
	q := &%[1]s{db: db}
	if err := q.Validate(); err != nil {
		return nil, err
	}
	return q, nil
}
`

func (g *Generator) printAdditionalImports() {
	for _, impt := range g.additionalImports {
		g.Printf("import \"%s\"\n", impt)
	}

	g.additionalImports = []string{}
}

func (g *Generator) build(fields []Field, typeName string) {
	g.printAdditionalImports()
	queryClass := fmt.Sprintf("%sQuery", typeName)
	queryTransactionClass := fmt.Sprintf("%sQueryTxn", typeName)
	tableName := strings.ToLower(typeName)
	log.Printf("Type: %s Fields: %v\n", typeName, fields)

	// -- Query definition BEGIN
	g.Printf("type %s struct {\n", queryClass)
	g.Printf("db *sql.DB\n")
	for _, field := range fields {
		g.Printf("by%s *sql.Stmt\n", field.srcName)
	}
	g.Printf("}\n")
	// -- Query definition END

	// -- Query transaction definition BEGIN
	g.Printf("type %s struct {\n", queryTransactionClass)
	g.Printf("tx *sql.Tx\n")
	g.Printf("q *%s", queryClass)
	g.Printf("}\n")
	// -- Query transaction definition END

	g.Printf(newQueryDefn, queryClass)

	var srcFieldPtrs bytes.Buffer
	var dbFieldNames bytes.Buffer
	for i, field := range fields {
		if i != 0 {
			srcFieldPtrs.WriteString(", ")
			dbFieldNames.WriteString(",")
		}
		srcFieldPtrs.WriteString(fmt.Sprintf("&obj.%s", field.srcName))
		dbFieldNames.WriteString(field.dbName)
	}

	// -- Validate method BEGIN
	g.Printf("func (q *%s) Validate() error {\n", queryClass)
	for _, field := range fields {
		g.Printf(`if stmt, err := q.db.Prepare("SELECT %s FROM %s WHERE %s = $1;"); err != nil {
			`, dbFieldNames.String(), tableName, field.dbName)
		g.Printf("return err\n")
		g.Printf("} else {\n")
		g.Printf("q.by%s = stmt\n", field.srcName)
		g.Printf("}\n")
	}
	g.Printf("return nil\n")
	g.Printf("}\n")
	// -- Validate method END

	// -- Create new transaction BEGIN
	g.Printf("func (q *%s) Transaction() (*%s, error) {\n", queryClass, queryTransactionClass)
	g.Printf("if tx, err := q.db.Begin(); err != nil {\n")
	g.Printf("return nil, err\n")
	g.Printf("} else {\n")
	g.Printf("return &%s{tx: tx, q: q}, nil\n", queryTransactionClass)
	g.Printf("}\n")
	g.Printf("}\n")
	// -- Create new transaction END

	for _, field := range fields {
		if field.isPK {
			g.Printf("func (tq *%s) By%s(%s %s) (*%s, error) {\n", queryTransactionClass, field.srcName, field.srcName, field.srcType, typeName)
			g.Printf("row := tq.tx.Stmt(tq.q.by%s).QueryRow(%s)\n", field.srcName, field.srcName)
			g.Printf("obj := new(%s)\n", typeName)
			g.Printf("if err := row.Scan(%s); err != nil {\n", srcFieldPtrs.String())
			g.Printf("return nil, err\n")
			g.Printf("}\n")
			g.Printf("return obj, nil\n")
		} else {
			// TODO: Returning channels is a slightly dangerous operation. There is a possibility this
			// channel will not be completely consumed by the receiver. In that case, close() never gets
			// called on the channels and causes a memory leak.
			g.Printf("func (tq *%s) By%s(%s %s) (chan<- *%s, chan<- error) {\n", queryTransactionClass, field.srcName, field.srcName, field.srcType, typeName)
			g.Printf("objChan := make(chan *%s, 10)\n", typeName)
			g.Printf("errChan := make(chan error, 10)\n")
			g.Printf("if rows, err := tq.tx.Stmt(tq.q.by%s).Query(%s); err != nil {\n", field.srcName, field.srcName)
			g.Printf("errChan <- err\n")
			g.Printf("} else {\n")
			g.Printf("go func() {\n")
			g.Printf("for rows.Next() {\n")
			g.Printf("obj := new(%s)\n", typeName)
			g.Printf("if err := rows.Scan(%s); err != nil {\n", srcFieldPtrs.String())
			g.Printf("errChan <- err\n")
			g.Printf("}\n")
			g.Printf("objChan <- obj")
			g.Printf("}\n")
			g.Printf("close(objChan)\n")
			g.Printf("close(errChan)\n")
			g.Printf("}()\n")
			g.Printf("}\n")
			g.Printf("return objChan, errChan")
		}
		g.Printf("}\n")
	}
}
