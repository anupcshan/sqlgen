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
		t.Fatalf("Expected imports: %s\nActual imports: %s\n", expectedImports, actualImports)
	}

	g = &Generator{
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
		t.Fatalf("Expected imports: %s\nActual imports: %s\n", expectedQueryDecl, actualQueryDecl)
	}
}
