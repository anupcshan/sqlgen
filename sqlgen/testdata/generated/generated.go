package foopackage

import "database/sql"

type TypeNameQuery struct {
	db *sql.DB
	create *sql.Stmt
	bysrcName *sql.Stmt
	bySrcName2 *sql.Stmt
}

type TypeNameQueryTx struct {
	tx *sql.Tx
	q *TypeNameQuery
}

func (q *TypeNameQuery) Validate() error {
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

func (q *TypeNameQuery) Create(obj *TypeName) error {
	if result, err := q.create.Exec(&obj.srcName, &obj.SrcName2); err != nil {
		return err
	} else {
		return nil
	}
}

func (q *TypeNameQuery) Transaction() (*TypeNameQueryTx, error) {
	if tx, err := q.db.Begin(); err != nil {
		return nil, err
	} else {
		return &TypeNameQueryTx{tx: tx, q: q}, nil
	}
}
