package sqlgen

import (
	"bytes"
	"fmt"
)

// Value represents a declared field.
type Field struct {
	srcName string // Field name in source
	dbName  string // Field name in DB
	isPK    bool   // Is the field a primary key?
	srcType string // Field type in source
	dbType  string // Expected field type in the DB
}

// Each struct type. Maps to one table in the database.
type Type struct {
	name      string  // Type name in source
	tableName string  // Table name in DB
	fields    []Field // List of fields synced with DB
	// TODO: Do we need a mechanism to refer to other tables?
}

type Generator struct {
	buf               bytes.Buffer // Output buffer
	additionalImports []string     // List of additional imports (for local data types)
	_type             Type         // Struct/table to be exported.
}

func (g *Generator) printImports() {
	g.Printfln(`import "database/sql"`)
	for _, impt := range g.additionalImports {
		g.Printfln(`import "%s"`, impt)
	}
}

func (g *Generator) Printfln(format string, args ...interface{}) {
	fmt.Fprintf(&g.buf, format, args...)
	g.buf.Write([]byte("\n"))
}

func (g *Generator) printQueryDeclaration() {
	g.Printfln("type %sQuery struct {", g._type.name)
	g.Printfln("db *sql.DB")
	g.Printfln("create *sql.Stmt")
	for _, field := range g._type.fields {
		g.Printfln("by%s *sql.Stmt", field.srcName)
	}
	g.Printfln("}")
}

func (g *Generator) Generate() {
	g.printImports()
	g.printQueryDeclaration()
}
