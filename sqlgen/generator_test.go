package sqlgen

import "testing"

var _type = Type{
	name:      "TypeName",
	tableName: "tblName",
	fields: []Field{
		Field{
			srcName: "srcName",
			dbName:  "dbName",
			isPK:    true,
			srcType: "int64",
			dbType:  "BIGINT",
		},
		Field{
			srcName: "SrcName2",
			dbName:  "dbName2",
			isPK:    false,
			srcType: "string",
			dbType:  "VARCHAR",
		},
	},
}

func TestPrintAdditionalImports(t *testing.T) {
	g := &Generator{additionalImports: []string{"time", "foo"}, sw: new(SourceWriter)}
	expectedImports := `import "database/sql"
import "time"
import "foo"
`
	g.printImports()
	if actualImports := g.sw.buf.String(); actualImports != expectedImports {
		t.Fatalf("Expected imports:\n%s\nActual imports:\n%s\n", expectedImports, actualImports)
	}
}

func TestQueryDeclaration(t *testing.T) {
	g := &Generator{
		additionalImports: []string{"time", "foo"},
		_type:             _type,
		sw:                new(SourceWriter),
	}

	expectedQueryDecl := `type TypeNameQuery struct {
	db *sql.DB
	create *sql.Stmt
	bysrcName *sql.Stmt
	bySrcName2 *sql.Stmt
}

type TypeNameQueryTx struct {
	tx *sql.Tx
	q *TypeNameQuery
}
`
	g.printQueryDeclaration()
	if actualQueryDecl := g.sw.buf.String(); actualQueryDecl != expectedQueryDecl {
		t.Fatalf("Expected query declaration:\n%s\nActual query declaration:\n%s\n", expectedQueryDecl, actualQueryDecl)
	}
}

func TestPrintSchemaValidation(t *testing.T) {
	g := &Generator{
		additionalImports: []string{"time", "foo"},
		_type:             _type,
		sw:                new(SourceWriter),
	}

	expectedSchemaVal := `func (q *TypeNameQuery) Validate() error {
	if stmt, err := q.db.Prepare("INSERT INTO tblName(dbName,dbName2) VALUES($1,$2)"); err != nil {
		return err
	}

	if stmt, err := q.db.Prepare("SELECT dbName,dbName2 FROM tblName WHERE dbName=$1"); err != nil {
		return err
	}

	if stmt, err := q.db.Prepare("SELECT dbName,dbName2 FROM tblName WHERE dbName2=$1"); err != nil {
		return err
	}

	return nil
}
`

	g.printSchemaValidation()
	if actualSchemaVal := g.sw.buf.String(); actualSchemaVal != expectedSchemaVal {
		t.Fatalf("Expected schema validation:\n%s\nActual schema validation:\n%s\n", expectedSchemaVal, actualSchemaVal)
	}
}

func TestCreateInstance(t *testing.T) {
	g := &Generator{
		additionalImports: []string{"time", "foo"},
		_type:             _type,
		sw:                new(SourceWriter),
	}

	expectedCreateInstStr := `func (q *TypeNameQuery) Create(obj *TypeName) error {
	if result, err := q.create.Exec(&obj.srcName, &obj.SrcName2); err != nil {
		return err
	}
	else {
		return nil
	}
}
`

	g.printCreateInstance()
	if actualCreateInstStr := g.sw.buf.String(); actualCreateInstStr != expectedCreateInstStr {
		t.Fatalf("Expected create instance str:\n%s\nActual create instance str:\n%s\n", expectedCreateInstStr, actualCreateInstStr)
	}
}
