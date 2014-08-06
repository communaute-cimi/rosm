package ws

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"regexp"
)

const (
	QUERY_INSERT_TILE_IN_CACHE = "INSERT INTO tiles (data, dthr, z, x, y) VALUES ($1, $2, $3, $4, $5)"
	QUERY_INSERT_LOG           = "INSERT INTO logs (action, msg, dthr, z, x, y) VALUES ($1, $2, $3, $4, $5, $6)"
	QUERY_TILE_EXIST           = "SELECT data FROM tiles WHERE z=$1 AND x=$2 AND y=$3"
	QUERY_COUNT_LOG            = "SELECT count(id) as nb FROM logs WHERE action = $1"
	QUERY_COUNT_TD_LOG         = "SELECT count(id) as nb FROM logs WHERE action = $1 and dthr >= (now() - '1 day'::INTERVAL)"
	QUERY_STATS_Z              = "SELECT count(z), z FROM logs WHERE action = $1 GROUP BY z ORDER BY count(z) DESC"
)

func WSHandler(db *sql.DB) http.Handler {

	type WSResultHitDay struct {
		Value int
		Msg   string
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := r.URL.Path[1:]
		re := regexp.MustCompile("^ws/(hitcache|hitday)/$")
		reresult := re.FindStringSubmatch(path)
		if reresult != nil {
			action := reresult[1]
			switch action {
			case "hitday":
				countCacheTD := cacheToday(db)
				wr := &WSResultHitDay{countCacheTD, "hit du jour"}
				wsres, err := json.Marshal(wr)
				if err != nil {
					log.Printf("Oups ... je serialize pas le hitday")
					http.Error(w, err.Error(), 500)
					return
				} else {
					log.Printf("intero ws hitday %s", wsres)
					w.Write(wsres)
					return
				}
			case "hitcache":
				fmt.Fprintf(w, "%s", "Not imp")
				return
			default:
				log.Printf("%s %s %s %s", r.Method, r.RemoteAddr, r.UserAgent(), r.URL.Path[1:])
				http.NotFound(w, r)
			}
		}
		fmt.Fprint(w, path)
	})
}

func cacheToday(db *sql.DB) int {
	rows, err := db.Query(QUERY_COUNT_TD_LOG, "cache")
	if err != nil {
		log.Print(err)
	}
	defer rows.Close()
	for rows.Next() {
		var nb int
		if err := rows.Scan(&nb); err != nil {
			log.Print(err)
		}
		return nb
	}
	if err := rows.Err(); err != nil {
		log.Print(err)
	}
	return 0
}
