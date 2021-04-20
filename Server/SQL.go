package main

import (
	"database/sql"
	"fmt"
	"log"
	"os"
	"strings"

	_ "github.com/go-sql-driver/mysql"
	_ "github.com/lib/pq"
	_ "github.com/mattn/go-sqlite3"
)

// Simple translation layer to allow some common vanilla SQL
// routines to be used in a number of databases. Main difference
// is the syntax for variables we pass into prepared statements:
//
// - MySQL : '?'
// - PostgreSQL : '$n' (n = 1,2,3,...)
// - SQLite : '?' or '$n'
// - Oracle : ':varname'
//
// We therefore store a set of customised SQL procedure strings,
// transformed according to the specific database we're using.
//
// Implementation is (and should be) opaque, so lower-case members.

type SQLShim struct {
	setup string
	lookup string
	insert string
	db *sql.DB
}

// Initialises stored SQL procedures for the specified database type
func (s *SQLShim) InitProcedures(dbType string) (error) {
	if dbType == "" { log.Fatalln("Database type is empty!") }

	//
	// Primary keys:
	// - MySql   : id int AUTO_INCREMENT PRIMARY KEY
	// - Postgres: id int GENERATED BY DEFAULT AS IDENTITY PRIMARY KEY
	// - SQLite  : id integer PRIMARY KEY
	// - Oracle  : (as Postgres)
	//
	// Select/Insert variables:
	// - MySql   : "?
	// - Postgres: "$N"
	// - SQLite  : Either "?" or "$N"
	// - Oracle  : ":name"
	//
	// Note: MySQL cannot use "text" as an unique index, as the length is
	// unbounded; we therefore use varchar() for barcode column.
	//

	const (
		rawSetup = `CREATE TABLE IF NOT EXISTS barcodes(
		id      %s          PRIMARY KEY,
		barcode varchar(50) NOT NULL UNIQUE,
		isbn    text        NOT NULL,
		author  text        NOT NULL,
		title   text        NOT NULL);`	

		rawLookup = "SELECT isbn,author,title FROM barcodes WHERE barcode=(?);"

		rawInsert = `INSERT INTO barcodes(barcode,isbn,author,title)
		SELECT ?,?,?,?
		WHERE NOT EXISTS (SELECT * FROM barcodes WHERE barcode=(?));`
	)

	varReplace := func(src string, varPrefix string) (string,error) {
		builder := strings.Builder {}
		varIndex := 1
		for _,r := range src {
			if r == '?' {
				_, err := builder.WriteString(fmt.Sprintf("%s%d",varPrefix,varIndex))
				if err != nil { return "",err }
				varIndex++
			} else {
				_, err := builder.WriteRune(r)
				if err != nil { return "",err }
			}
		}
		return builder.String(), nil
	}

	// Modified according to database type
	idInfo := "int GENERATED BY DEFAULT AS IDENTITY"
	varPrefix := ""

	switch strings.ToLower(dbType) {
		case "mysql":
			idInfo = "int AUTO_INCREMENT"
		case "sqlite":
			idInfo = "integer"
		case "postgres":
			// Postgres will get SELECT variables as $1, $2, ...
			varPrefix = "$"
		default:
			return fmt.Errorf("Unknown database type " + dbType)
//		case "oracle":
//			// Oracle will get SELECT variables as :var1, :var2, ...
//			varPrefix = ":var"
	}

	s.setup = fmt.Sprintf(rawSetup, idInfo)
	s.lookup, s.insert = rawLookup, rawInsert

	if varPrefix != "" {
		lookup, err := varReplace(rawLookup,varPrefix)
		if err != nil {return err}

		insert, err := varReplace(rawInsert,varPrefix)
		if err != nil {return err}

		s.lookup, s.insert = lookup, insert
	}

	/*
	log.Println("SQL strings for database type " + dbType + ":")
	log.Println(" - Setup: " + s.setup)
	log.Println(" - Lookup: " + s.lookup)
	log.Println(" - Insert: " + s.insert)
	*/

	return nil
}

// Sets the internal SQL database object and executes the SQL setup procedure
func (s *SQLShim) SetupDatabase(db *sql.DB) (error) {
	if db == nil { log.Fatalln("Database is nil!") }

	s.db = db

	_, err := s.db.Exec(s.setup)
	return err
}

// Returns a BarcodeItem from the database
func (s *SQLShim) Lookup(barcode string) (*BarcodeItem, error) {
	if s.db == nil { log.Fatalln("Database is nil!") }
	if barcode == "" { log.Fatalln("Empty barcode!") }

	rows, err := s.db.Query(s.lookup,barcode)
	if err != nil { return nil, err }

	defer rows.Close()

	for rows.Next() {
		tmp := BarcodeItem {Barcode: barcode}

		err := rows.Scan(&tmp.ISBN,&tmp.Author,&tmp.Title)
		if err != nil { return nil, err }

		return &tmp, nil
	}

	return nil, nil
}

// Stores BarcodeItem in the database
func (s *SQLShim) Store(item *BarcodeItem) (error) {
	if s.db == nil { log.Fatalln("Database is nil!") }
	if item == nil { log.Fatalln("Item is nil!") }
	if item.Barcode == "" { log.Fatalln("Barcode is empty!") }

	_, err := s.db.Exec(s.insert,
		item.Barcode,
		item.ISBN,
		item.Author,
		item.Title,
		item.Barcode )
	
	return err
}


//
// SQLite
//

type SQLiteServer struct {
	shim SQLShim
}

// params = SQLite file path
func (s *SQLiteServer) Startup(params string) {
	s.Shutdown()

	filePath := params

	info, err := os.Stat(filePath)
	if os.IsNotExist(err) || info.IsDir() {
		log.Println("Database file '"+filePath+"' does not exist; creating ...")
		os.Create(filePath)
	}

	db, err := sql.Open("sqlite3",filePath)
	boom(err, "Unable to open SQLite database "+filePath)

	err = s.shim.InitProcedures("SQLite")
	boom(err, "Unable to initialize procedures")

	err = s.shim.SetupDatabase(db)
	boom(err, "Unable to set up database ")
}

// Closes internal database object
func (s *SQLiteServer) Shutdown() {
	if s.shim.db != nil { s.shim.db.Close() }
	s.shim.db = nil
}

// Returns a BarcodeItem from the database
func (s *SQLiteServer) Lookup(barcode string) (*BarcodeItem) {
	result, err := s.shim.Lookup(barcode)
	boom(err, "Unable to lookup item")
	return result
}

// Stores a BarcodeItem in the database
func (s *SQLiteServer) Store(item *BarcodeItem) {
	err := s.shim.Store(item)
	boom(err, "Unable to store item")
}

//
// Postgres
//

type PostgresServer struct {
	shim SQLShim
}

// params = Postgres connection string
func (s *PostgresServer) Startup(params string) {
	s.Shutdown()

	what, connStr := "postgres", params

	db, err := sql.Open(what,connStr)
	boom(err, "Unable to open "+what+" database "+connStr)

	err = s.shim.InitProcedures(what)
	boom(err, "Unable to initialize procedures")

	err = s.shim.SetupDatabase(db)
	boom(err, "Unable to set up database ")
}

// Closes internal database object
func (s *PostgresServer) Shutdown() {
	if s.shim.db != nil { s.shim.db.Close() }
	s.shim.db = nil
}

// Returns a BarcodeItem from the database
func (s *PostgresServer) Lookup(barcode string) (*BarcodeItem) {
	result, err := s.shim.Lookup(barcode)
	boom(err, "Unable to lookup item")
	return result
}

// Stores a BarcodeItem in the database
func (s *PostgresServer) Store(item *BarcodeItem) {
	log.Println(item)
	err := s.shim.Store(item)
	boom(err,"Unable to store item")
}

//
// MySQL
//

type MySQLServer struct {
	shim SQLShim
}

// params = MySQL connection string
func (s *MySQLServer) Startup(params string) {
	s.Shutdown()

	what, connStr := "mysql", params

	db, err := sql.Open(what,connStr)
	boom(err, "Unable to open "+what+" database "+connStr)

	err = s.shim.InitProcedures(what)
	boom(err, "Unable to initialize procedures")

	err = s.shim.SetupDatabase(db)
	boom(err, "Unable to set up database ")
}

// Closes internal database object
func (s *MySQLServer) Shutdown() {
	if s.shim.db != nil { s.shim.db.Close() }
	s.shim.db = nil
}

// Returns a BarcodeItem from the database
func (s *MySQLServer) Lookup(barcode string) (*BarcodeItem) {
	result, err := s.shim.Lookup(barcode)
	boom(err, "Unable to lookup item")
	return result
}

// Stores a BarcodeItem in the database
func (s *MySQLServer) Store(item *BarcodeItem) {
	err := s.shim.Store(item)
	boom(err,"Unable to store item")
}
