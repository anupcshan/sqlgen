package sqlgen

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"strings"
	"testing"
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
	packageName: "foopackage",
}

// TODO: Tests
func stringDelta(expected string, actual string) string {
	eLines := strings.Split(expected, "\n")
	aLines := strings.Split(actual, "\n")
	var output bytes.Buffer
	i := 0
	seenFirstError := false

	for i = 0; i < len(eLines) && i < len(aLines); i++ {
		if eLines[i] == aLines[i] {
			if !seenFirstError {
				continue
			}
			output.WriteString("   ")
			output.WriteString(eLines[i])
			output.WriteByte('\n')
		} else {
			if !seenFirstError {
				output.WriteString(fmt.Sprintf("First error seen at line %d:\n", i+1))
				seenFirstError = true
			}
			output.WriteString("-- ")
			output.WriteString(eLines[i])
			output.WriteByte('\n')
			output.WriteString("++ ")
			output.WriteString(aLines[i])
			output.WriteByte('\n')
		}
	}

	for i = i; i < len(eLines); i++ {
		output.WriteString("-- ")
		output.WriteString(eLines[i])
		output.WriteByte('\n')
	}

	for i = i; i < len(aLines); i++ {
		output.WriteString("++ ")
		output.WriteString(aLines[i])
		output.WriteByte('\n')
	}

	return output.String()
}

func TestPrintAdditionalImports(t *testing.T) {
	g := &Generator{additionalImports: []string{"time", "foo"}, sw: new(SourceWriter), _type: Type{packageName: "fpkg"}}
	expectedImports := `package fpkg

import "database/sql"
import "time"
import "foo"
`
	g.printFileHeader()
	if actualImports := g.sw.buf.String(); actualImports != expectedImports {
		t.Fatalf("Mismatch in imports str:\n%s\n", stringDelta(expectedImports, actualImports))
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
	update *sql.Stmt
}

type TypeNameQueryTx struct {
	tx *sql.Tx
	q *TypeNameQuery
}
`
	g.printQueryDeclaration()
	if actualQueryDecl := g.sw.buf.String(); actualQueryDecl != expectedQueryDecl {
		t.Fatalf("Mismatch in query declaration str:\n%s\n", stringDelta(expectedQueryDecl, actualQueryDecl))
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
	} else {
		q.create = stmt
	}

	if stmt, err := q.db.Prepare("SELECT dbName,dbName2 FROM tblName WHERE dbName=$1"); err != nil {
		return err
	} else {
		q.bysrcName = stmt
	}

	if stmt, err := q.db.Prepare("SELECT dbName,dbName2 FROM tblName WHERE dbName2=$1"); err != nil {
		return err
	} else {
		q.bySrcName2 = stmt
	}

	if stmt, err := q.db.Prepare("UPDATE tblName SET (dbName2)=($2) WHERE dbName=$1"); err != nil {
		return err
	} else {
		q.update = stmt
	}

	return nil
}
`

	g.printSchemaValidation()
	if actualSchemaVal := g.sw.buf.String(); actualSchemaVal != expectedSchemaVal {
		t.Fatalf("Mismatch in schema validation str:\n%s\n", stringDelta(expectedSchemaVal, actualSchemaVal))
	}
}

func TestCreateInstance(t *testing.T) {
	g := &Generator{
		additionalImports: []string{"time", "foo"},
		_type:             _type,
		sw:                new(SourceWriter),
	}

	expectedCreateInstStr := `func (t *TypeNameQueryTx) Create(obj *TypeName) error {
	stmt := t.tx.Stmt(t.q.create)
	if _, err := stmt.Exec(&obj.srcName, &obj.SrcName2); err != nil {
		return err
	} else {
		return nil
	}
}

func (t *TypeNameQueryTx) Update(obj *TypeName) error {
	stmt := t.tx.Stmt(t.q.update)
	if _, err := stmt.Exec(&obj.srcName, &obj.SrcName2); err != nil {
		return err
	} else {
		return nil
	}
}
`

	g.printInstanceCU()
	if actualCreateInstStr := g.sw.buf.String(); actualCreateInstStr != expectedCreateInstStr {
		t.Fatalf("Mismatch in create instance str:\n%s\n", stringDelta(expectedCreateInstStr, actualCreateInstStr))
	}
}

func TestCreateTransaction(t *testing.T) {
	g := &Generator{
		additionalImports: []string{"time", "foo"},
		_type:             _type,
		sw:                new(SourceWriter),
	}

	expectedCreateTxnStr := `func (q *TypeNameQuery) Transaction() (*TypeNameQueryTx, error) {
	if tx, err := q.db.Begin(); err != nil {
		return nil, err
	} else {
		return &TypeNameQueryTx{tx: tx, q: q}, nil
	}
}

func (t *TypeNameQueryTx) Commit() error {
	return t.tx.Commit()
}

func (t *TypeNameQueryTx) Rollback() error {
	return t.tx.Rollback()
}
`

	g.printCreateTransaction()
	if actualCreateTxnStr := g.sw.buf.String(); actualCreateTxnStr != expectedCreateTxnStr {
		t.Fatalf("Mismatch in create txn str:\n%s\n", stringDelta(expectedCreateTxnStr, actualCreateTxnStr))
	}
}

func TestGenerate(t *testing.T) {
	g := &Generator{
		// additionalImports: []string{"time", "foo"},
		_type: _type,
		sw:    new(SourceWriter),
	}

	expectedBytes, _ := ioutil.ReadFile("testdata/generated.go")
	expectedStr := string(expectedBytes)
	g.Generate()
	if actualStr := g.sw.buf.String(); actualStr != expectedStr {
		t.Fatalf("Mismatch in file contents:\n%s\n", stringDelta(expectedStr, actualStr))
	}
}
