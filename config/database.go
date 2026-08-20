package config

import (
	"database/sql"
	"fmt"
	"os"
	"time"

	"log/slog"
	_ "modernc.org/sqlite"
)

type logger interface {
	Debug(msg string, args ...any)
	Info(msg string, args ...any)
	Error(msg string, args ...any)
}

func OpenDatabase(log *slog.Logger, sqlFile string) (*sql.DB, error) {
	if _, err := os.Stat(sqlFile); os.IsNotExist(err) {
		log.Info("Creating database")

		file, err := os.Create(sqlFile) // Create SQLite file
		if err != nil {
			return nil, fmt.Errorf("create database file: %w", err)
		}

		err = file.Close()
		if err != nil {
			return nil, fmt.Errorf("close new database file: %w", err)
		}

		log.Info("Database created")
	}

	db, err := sql.Open("sqlite", sqlFile)
	if err != nil {
		return nil, fmt.Errorf("open database: %w", err)
	}

	defer func(db *sql.DB) {
		err := db.Close()
		if err != nil {
			log.Error("Closing the database.", err)
		}
	}(db)

	// To resolve too many files open error
	// If this does not work, then perform a db.close and then open/close for each query.
	// db.SetMaxOpenConns(10)
	// db.SetMaxIdleConns(5)
	// db.SetConnMaxIdleTime(30 * time.Second)

	dbTables := db.QueryRow("SELECT name FROM sqlite_master WHERE type='table' AND name='fileHash';")
	if err := dbTables.Scan(); err == sql.ErrNoRows {
		log.Info("Prepping database.")
		query := `CREATE TABLE fileHash (
			"hash" TEXT NOT NULL PRIMARY KEY,
			"filename" TEXT,
			"lastChecked" INTEGER
		  );` // SQL Statement for Create Table

		if _, err := db.Exec(query); err != nil {
			return nil, fmt.Errorf("creating hash table: %w", err)
		}
	}

	return db, nil
}

func SaveHash(log *slog.Logger, dbFile string, hash string, fileName string) error {
	db, err := sql.Open("sqlite", dbFile)
	if err != nil {
		return fmt.Errorf("open database: %w", err)
	}

	defer func(db *sql.DB) {
		err := db.Close()
		if err != nil {
			log.Error("Could not close the database.", err)
		}
	}(db)

	query := `INSERT OR REPLACE INTO fileHash(hash, fileName, lastChecked)
			VALUES(?, ?, ?);`
	if _, err = executeSQL(db, query,
		hash,
		fileName,
		time.Now().Unix(),
	); err != nil {
		return fmt.Errorf("saving hash to database: %w", err)
	}

	log.Info("Saved hash to database.", slog.String("Hash", hash))
	return nil
}

func LookupHash(log logger, dbFile string, hash string) (bool, error) {
	db, err := sql.Open("sqlite", dbFile)
	if err != nil {
		return false, fmt.Errorf("open database: %w", err)
	}

	defer func(db *sql.DB) {
		err := db.Close()
		if err != nil {
			log.Error("Could not close the database.", err)
		}
	}(db)

	query := `SELECT hash, fileName, lastChecked
				FROM fileHash
				WHERE hash=?;`
	rows, err := executeSQL(db, query, hash)
	if err != nil {
		return false, fmt.Errorf("checking database for hash [%s]: %w", hash, err)
	}

	defer func(rows *sql.Rows) {
		if rows.Close() != nil {
			log.Error("Could not close DB Rows.", err)
		}
	}(rows)

	var (
    dbHash      string
    fileName    string
    lastChecked int64
  )

  if rows.Next() {
    err = rows.Scan(&dbHash, &fileName, &lastChecked)
    if err != nil {
      return false, fmt.Errorf("read DB row: %w", err)
    }

    log.Info("Found match!",
      slog.String("Hash", dbHash),
      slog.String("TimeStamp", time.Unix(lastChecked, 0).Format("01/02/06 15:04:05")),
    )

    return true, nil
  }

  return false, nil
}

func executeSQL(db *sql.DB, query string, values ...interface{}) (*sql.Rows, error) {
	statement, err := db.Prepare(query) // Prepare SQL Statement
	if err != nil {
		return &sql.Rows{}, fmt.Errorf("prepare statement [%s]: %w", query, err)
	}

	result, err := statement.Query(values...) // Execute SQL Statements
	if err != nil {
		return &sql.Rows{}, fmt.Errorf("execute query [%s]: %w", query, err)
	}

	// if err := db.Close(); err != nil {
	// 	return &sql.Rows{}, fmt.Errorf("failed to close the database after operation: %w", err)
	// }

	return result, nil
}
