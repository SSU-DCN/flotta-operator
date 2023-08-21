// database/db.go
package database

import (
	"database/sql"

	_ "github.com/lib/pq"
	"github.com/project-flotta/flotta-operator/internal/edgeapi"
)

var Config edgeapi.Config
var (
	db *sql.DB
)

func InitConnection(DBUser, DBPassword, DBHost, DBPort, DBName, SSLMode string) error {
	ConnectionInfo := "user=" + DBUser + " password=" + DBPassword + " host=" + DBHost + " port=" + DBPort + " dbname=" + DBName + " sslmode=" + SSLMode
	var err error
	db, err = sql.Open("postgres", ConnectionInfo)
	if err != nil {
		return err
	}

	err = db.Ping()
	if err != nil {
		return err
	}

	return nil
}

func GetDB() *sql.DB {
	return db
}
