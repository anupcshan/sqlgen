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

type SourceWriter struct {
	buf         bytes.Buffer
	indentLevel int
}

func (s *SourceWriter) Indent() *SourceWriter {
	s.indentLevel++
	return s
}

func (s *SourceWriter) Unindent() *SourceWriter {
	if s.indentLevel == 0 {
		panic("Cannot unindent from left edge.")
	}
	s.indentLevel--
	return s
}

type CompoundStatement struct {
	sw *SourceWriter
}

func (s *SourceWriter) NewCompoundStatement(format string, args ...interface{}) *CompoundStatement {
	s.Printf(format, args...)
	s.buf.Write([]byte(" {\n"))
	s.Indent()

	return &CompoundStatement{sw: s}
}

func (cs *CompoundStatement) Printfln(format string, args ...interface{}) *CompoundStatement {
	cs.sw.Printfln(format, args...)
	return cs
}

func (cs *CompoundStatement) NewCompoundStatement(format string, args ...interface{}) *CompoundStatement {
	return cs.sw.NewCompoundStatement(format, args...)
}

func (cs *CompoundStatement) AddNewline() *CompoundStatement {
	cs.sw.AddNewline()
	return cs
}

func (cs *CompoundStatement) Close() *SourceWriter {
	cs.sw.Unindent()
	cs.sw.Printfln("}")
	return cs.sw
}

func (s *SourceWriter) Printf(format string, args ...interface{}) *SourceWriter {
	for i := 0; i < s.indentLevel; i++ {
		s.buf.WriteByte('\t')
	}
	fmt.Fprintf(&s.buf, format, args...)
	return s
}

func (s *SourceWriter) Printfln(format string, args ...interface{}) *SourceWriter {
	return s.Printf(format, args...).AddNewline()
}

func (s *SourceWriter) AddNewline() *SourceWriter {
	s.buf.WriteByte('\n')
	return s
}

type Generator struct {
	sw                *SourceWriter // Output buffer
	additionalImports []string      // List of additional imports (for local data types)
	_type             Type          // Struct/table to be exported.
}

func (g *Generator) printImports() {
	g.sw.Printfln(`import "database/sql"`)
	for _, impt := range g.additionalImports {
		g.sw.Printfln(`import "%s"`, impt)
	}
}

func (g *Generator) printQueryDeclaration() {
	// -- Query definition BEGIN
	{
		cs := g.sw.NewCompoundStatement("type %sQuery struct", g._type.name)
		cs.
			Printfln("db *sql.DB").
			Printfln("create *sql.Stmt")
		for _, field := range g._type.fields {
			cs.Printfln("by%s *sql.Stmt", field.srcName)
		}
		cs.Close()
	}
	// -- Query definition END

	g.sw.AddNewline()

	// -- Query transaction definition BEGIN
	{
		queryTransactionClass := fmt.Sprintf("%sQueryTx", g._type.name)
		cs := g.sw.NewCompoundStatement("type %s struct", queryTransactionClass)
		cs.
			Printfln("tx *sql.Tx").
			Printfln("q *%sQuery", g._type.name).
			Close()
	}
	// -- Query transaction definition END
}

func (g *Generator) printSchemaValidation() {
	var srcFieldPtrs bytes.Buffer
	var dbFieldNames bytes.Buffer
	var placeholders bytes.Buffer
	for i, field := range g._type.fields {
		if i != 0 {
			srcFieldPtrs.WriteString(", ")
			dbFieldNames.WriteString(",")
			placeholders.WriteString(",")
		}
		srcFieldPtrs.WriteString(fmt.Sprintf("&obj.%s", field.srcName))
		dbFieldNames.WriteString(field.dbName)
		placeholders.WriteString(fmt.Sprintf("$%d", i+1))
	}

	method := g.sw.NewCompoundStatement("func (q *%sQuery) Validate() error", g._type.name)
	cs := method.NewCompoundStatement(`if stmt, err := q.db.Prepare("INSERT INTO %s(%s) VALUES(%s)"); err != nil`,
		g._type.tableName, dbFieldNames.String(), placeholders.String())
	cs.
		Printfln("return err").
		Close()

	for _, field := range g._type.fields {
		// TODO: Ideally, this newline would be added automatically.
		method.AddNewline()

		cs := method.NewCompoundStatement(`if stmt, err := q.db.Prepare("SELECT %s FROM %s WHERE %s=$1"); err != nil`,
			dbFieldNames.String(), g._type.tableName, field.dbName)
		cs.
			Printfln("return err").
			Close()
	}

	// TODO: Ideally, this newline would be added automatically.
	method.AddNewline()

	method.
		Printfln("return nil").
		Close()
}

func (g *Generator) Generate() {
	g.printImports()
	g.printQueryDeclaration()
	g.printSchemaValidation()
}
