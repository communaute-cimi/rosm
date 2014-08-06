package utils

import (
	"database/sql"
	"log"
	"time"
)

func CheckDB(db *sql.DB) {
	if r := recover(); r != nil {
		log.Printf("recovered %s", r)
		pingcnx(db)
	}
}

func pingcnx(db *sql.DB) {
	// je trouve ca moche mais ca permet de pas stop l'appli en
	// cas de perte de cnx avec pg. voir avec vincent le coût sur la db
	for {
		err := db.Ping()
		if err != nil {
			time.Sleep(5 * time.Second)
			log.Printf("check pgsql cnx (%s) ...", err)
		} else {
			break
		}
	}
}
