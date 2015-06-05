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
		cs.Printfln("delete *sql.Stmt")
		cs.Printfln("update *sql.Stmt")
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
	var nonPKDbFieldNames bytes.Buffer
	var nonPKPlaceholders bytes.Buffer

	var pKDbFieldNames bytes.Buffer
	var pKPlaceholders bytes.Buffer

	var dbFieldNames bytes.Buffer
	var placeholders bytes.Buffer
	for i, field := range g._type.fields {
		if i != 0 {
			dbFieldNames.WriteString(",")
			placeholders.WriteString(",")
		}

		dbFieldNames.WriteString(field.dbName)
		placeholders.WriteString(fmt.Sprintf("$%d", i+1))
		if field.isPK {
			if pKDbFieldNames.Len() != 0 {
				panic("Multiple primary key columns not implemented.")
			}

			pKDbFieldNames.WriteString(field.dbName)
			pKPlaceholders.WriteString(fmt.Sprintf("$%d", i+1))
		} else {
			if nonPKDbFieldNames.Len() != 0 {
				nonPKDbFieldNames.WriteString(",")
				nonPKPlaceholders.WriteString(",")
			}

			nonPKDbFieldNames.WriteString(field.dbName)
			nonPKPlaceholders.WriteString(fmt.Sprintf("$%d", i+1))
		}
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

	method.NewCompoundStatement(`if stmt, err := q.db.Prepare("UPDATE %s SET (%s)=(%s) WHERE %s=%s"); err != nil`,
		g._type.tableName, nonPKDbFieldNames.String(), nonPKPlaceholders.String(), pKDbFieldNames.String(), pKPlaceholders.String()).
		Printfln("return err").
		CloseAndReopen("else").
		Printfln("q.update = stmt").
		Close()

	method.AddNewline()

	method.NewCompoundStatement(`if stmt, err := q.db.Prepare("DELETE FROM %s WHERE %s=%s"); err != nil`,
		g._type.tableName, pKDbFieldNames.String(), pKPlaceholders.String()).
		Printfln("return err").
		CloseAndReopen("else").
		Printfln("q.delete = stmt").
		Close()

	method.AddNewline()

	method.
		Printfln("return nil").
		Close()
}

func (g *Generator) printInstanceCUD() {
	var srcFieldPtrs bytes.Buffer
	var pkSrcFieldPtrs bytes.Buffer
	for i, field := range g._type.fields {
		if i != 0 {
			srcFieldPtrs.WriteString(", ")
		}
		srcFieldPtrs.WriteString(fmt.Sprintf("&obj.%s", field.srcName))

		if field.isPK {
			if pkSrcFieldPtrs.Len() != 0 {
				panic("Multiple primary key columns not implemented.")
			}
			pkSrcFieldPtrs.WriteString(fmt.Sprintf("&obj.%s", field.srcName))
		}
	}

	method := g.sw.NewCompoundStatement("func (t *%[1]sQueryTx) Create(obj *%[1]s) error", g._type.name)
	method.
		Printfln("stmt := t.tx.Stmt(t.q.create)").
		NewCompoundStatement("if _, err := stmt.Exec(%s); err != nil", srcFieldPtrs.String()).
		Printfln("return err").
		CloseAndReopen("else").
		Printfln("return nil").
		Close()
	method.Close()

	g.sw.AddNewline()
	method = g.sw.NewCompoundStatement("func (t *%[1]sQueryTx) Update(obj *%[1]s) error", g._type.name)
	method.
		Printfln("stmt := t.tx.Stmt(t.q.update)").
		NewCompoundStatement("if _, err := stmt.Exec(%s); err != nil", srcFieldPtrs.String()).
		Printfln("return err").
		CloseAndReopen("else").
		Printfln("return nil").
		Close()
	method.Close()

	g.sw.AddNewline()
	method = g.sw.NewCompoundStatement("func (t *%[1]sQueryTx) Delete(obj *%[1]s) error", g._type.name)
	method.
		Printfln("stmt := t.tx.Stmt(t.q.delete)").
		NewCompoundStatement("if _, err := stmt.Exec(%s); err != nil", pkSrcFieldPtrs.String()).
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

	g.sw.AddNewline()
	g.sw.NewCompoundStatement("func (t *%[1]sQueryTx) Commit() error", g._type.name).
		Printfln("return t.tx.Commit()").
		Close()

	g.sw.AddNewline()
	g.sw.NewCompoundStatement("func (t *%[1]sQueryTx) Rollback() error", g._type.name).
		Printfln("return t.tx.Rollback()").
		Close()
}

func (g *Generator) Generate() {
	g.printFileHeader()
	g.sw.AddNewline()
	g.printQueryDeclaration()
	g.sw.AddNewline()
	g.printSchemaValidation()
	g.sw.AddNewline()
	g.printCreateTransaction()
	g.sw.AddNewline()
	g.printInstanceCUD()
	g.sw.Format()
}
