package sqlgen

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

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
	g := &Generator{additionalImports: []string{"time", "foo"}}
	expectedImports := `import "database/sql"
import "time"
import "foo"
`
	g.printImports()
	assert.Equal(t, expectedImports, g.buf.String())

	g = &Generator{
		additionalImports: []string{"time", "foo"},
		_type:             _type}

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
	assert.Equal(t, expectedQueryDecl, g.buf.String())
}
