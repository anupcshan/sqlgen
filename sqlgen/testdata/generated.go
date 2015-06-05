package foopackage

import "database/sql"

type TypeNameQuery struct {
	db         *sql.DB
	create     *sql.Stmt
	bysrcName  *sql.Stmt
	bySrcName2 *sql.Stmt
	delete     *sql.Stmt
	update     *sql.Stmt
}

type TypeNameQueryTx struct {
	tx *sql.Tx
	q  *TypeNameQuery
}

func (q *TypeNameQuery) Validate() error {
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

	if stmt, err := q.db.Prepare("DELETE FROM tblName WHERE dbName=$1"); err != nil {
		return err
	} else {
		q.delete = stmt
	}

	return nil
}

func (q *TypeNameQuery) Transaction() (*TypeNameQueryTx, error) {
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

func (t *TypeNameQueryTx) Create(obj *TypeName) error {
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

func (t *TypeNameQueryTx) Delete(obj *TypeName) error {
	stmt := t.tx.Stmt(t.q.delete)
	if _, err := stmt.Exec(&obj.srcName); err != nil {
		return err
	} else {
		return nil
	}
}
