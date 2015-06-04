package sqlgen

import (
	"bytes"
	"fmt"
	"go/format"
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
	name        string  // Type name in source
	tableName   string  // Table name in DB
	fields      []Field // List of fields synced with DB
	packageName string  // Package that new type should go into
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

func (s *SourceWriter) newCompoundStatement(initialIndent bool, format string, args ...interface{}) *CompoundStatement {
	if initialIndent {
		s.Printf(format, args...)
	} else {
		s.printf(format, args...)
	}
	s.buf.Write([]byte(" {\n"))
	s.Indent()

	return &CompoundStatement{sw: s}
}

func (s *SourceWriter) NewCompoundStatement(format string, args ...interface{}) *CompoundStatement {
	return s.newCompoundStatement(true, format, args...)
}

func (s *SourceWriter) ContinueCompoundStatement(format string, args ...interface{}) *CompoundStatement {
	return s.newCompoundStatement(false, format, args...)
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

func (cs *CompoundStatement) CloseAndReopen(format string, args ...interface{}) *CompoundStatement {
	cs.sw.Unindent()
	cs.sw.Printf("} ")
	return cs.sw.ContinueCompoundStatement(format, args...)
}

func (cs *CompoundStatement) Close() *SourceWriter {
	cs.sw.Unindent()
	cs.sw.Printfln("}")
	return cs.sw
}

func (s *SourceWriter) PrintIndentation() *SourceWriter {
	for i := 0; i < s.indentLevel; i++ {
		s.buf.WriteByte('\t')
	}
	return s
}

func (s *SourceWriter) printf(format string, args ...interface{}) *SourceWriter {
	fmt.Fprintf(&s.buf, format, args...)
	return s
}

func (s *SourceWriter) Printf(format string, args ...interface{}) *SourceWriter {
	return s.PrintIndentation().printf(format, args...)
}

func (s *SourceWriter) Printfln(format string, args ...interface{}) *SourceWriter {
	return s.Printf(format, args...).AddNewline()
}

func (s *SourceWriter) AddNewline() *SourceWriter {
	s.buf.WriteByte('\n')
	return s
}

func (s *SourceWriter) Format() error {
	formattedBytes, err := format.Source(s.buf.Bytes())
	if err != nil {
		return err
	}

	s.buf.Reset()
	s.buf.Write(formattedBytes)
	return nil
}

type Generator struct {
	sw                *SourceWriter // Output buffer
	additionalImports []string      // List of additional imports (for local data types)
	_type             Type          // Struct/table to be exported.
}

func (g *Generator) printFileHeader() {
	g.sw.Printfln("package %s", g._type.packageName)
	g.sw.AddNewline()
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
	method.NewCompoundStatement(`if stmt, err := q.db.Prepare("INSERT INTO %s(%s) VALUES(%s)"); err != nil`,
		g._type.tableName, dbFieldNames.String(), placeholders.String()).
		Printfln("return err").
		CloseAndReopen("else").
		Printfln("q.create = stmt").
		Close()

	for _, field := range g._type.fields {
		// TODO: Ideally, this newline would be added automatically.
		method.AddNewline()

		method.NewCompoundStatement(`if stmt, err := q.db.Prepare("SELECT %s FROM %s WHERE %s=$1"); err != nil`,
			dbFieldNames.String(), g._type.tableName, field.dbName).
			Printfln("return err").
			CloseAndReopen("else").
			Printfln("q.by%s = stmt", field.srcName).
			Close()
	}

	// TODO: Ideally, this newline would be added automatically.
	method.AddNewline()

	method.
		Printfln("return nil").
		Close()
}

func (g *Generator) printCreateInstance() {
	var srcFieldPtrs bytes.Buffer
	for i, field := range g._type.fields {
		if i != 0 {
			srcFieldPtrs.WriteString(", ")
		}
		srcFieldPtrs.WriteString(fmt.Sprintf("&obj.%s", field.srcName))
	}

	method := g.sw.NewCompoundStatement("func (q *%[1]sQuery) Create(obj *%[1]s) error", g._type.name)
	method.
		NewCompoundStatement("if _, err := q.create.Exec(%s); err != nil", srcFieldPtrs.String()).
		Printfln("return err").
		CloseAndReopen("else").
		Printfln("return nil").
		Close()
	method.Close()
}

func (g *Generator) printCreateTransaction() {
	method := g.sw.NewCompoundStatement("func (q *%[1]sQuery) Transaction() (*%[1]sQueryTx, error)", g._type.name)
	method.NewCompoundStatement("if tx, err := q.db.Begin(); err != nil").
		Printfln("return nil, err").
		CloseAndReopen("else").
		Printfln("return &%sQueryTx{tx: tx, q: q}, nil", g._type.name).
		Close()
	method.Close()
}

func (g *Generator) Generate() {
	g.printFileHeader()
	g.sw.AddNewline()
	g.printQueryDeclaration()
	g.sw.AddNewline()
	g.printSchemaValidation()
	g.sw.AddNewline()
	g.printCreateInstance()
	g.sw.AddNewline()
	g.printCreateTransaction()
	g.sw.Format()
}
