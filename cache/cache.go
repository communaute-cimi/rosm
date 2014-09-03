package cache

import (
	"database/sql"
	"fmt"
	//	"github.com/communaute-cimi/rosm/utils"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"time"
)

const (
	QUERY_INSERT_TILE_IN_CACHE = "INSERT INTO tiles (data, dthr, z, x, y, src) VALUES ($1, $2, $3, $4, $5, $6)"
	QUERY_UPDATE_TILE_IN_CACHE = "UPDATE tiles set (data, dthr, src) = ($1, $2, $3) where z = $4 and x = $5 and y = $6"
	QUERY_INSERT_LOG           = "INSERT INTO logs (action, msg, dthr, z, x, y) VALUES ($1, $2, $3, $4, $5, $6)"
	QUERY_TILE_EXIST           = "SELECT data,dthr FROM tiles WHERE z=$1 AND x=$2 AND y=$3"
	QUERY_COUNT_LOG            = "SELECT count(id) as nb FROM logs WHERE action = $1"
	QUERY_COUNT_TD_LOG         = "SELECT count(id) as nb FROM logs WHERE action = $1 and dthr >= (now() - '1 day'::INTERVAL)"
	QUERY_STATS_Z              = "SELECT count(z), z FROM logs WHERE action = $1 GROUP BY z ORDER BY count(z) DESC"
)

type SrcOSM struct {
	Urlwww    string
	Httpproxy string
}

// Exemple d'autre Source pour tuille
type SrcFRASTER struct {
	Version string
	Url     string
}

type Tile struct {
	Z, X, Y int
	Data    []byte
	Dthr    time.Time
	Ttl     int
	Source  interface{}
}

type DbStorage struct {
	Db *sql.DB
}

// Exemple de storage
type BetaStorage struct {
	Name string
}

type Storage struct {
	Name  string
	Store interface{}
}

// Rajouter de l'abstraction dans la source de donnée (ftp, http ...)
type Cache interface {
	Get(*Tile) error
	Put(*Tile) error
}

func (s *Storage) Put(t *Tile) error {
	switch s.Store.(type) {
	case DbStorage:
		_, err := s.Store.(DbStorage).Db.Exec(QUERY_INSERT_TILE_IN_CACHE, t.Data, time.Now(), t.Z, t.X, t.Y, t.Source.(SrcOSM).Urlwww)
		if err != nil {
			return fmt.Errorf("err insert z:%d x:%d y:%d in cache %s", t.Z, t.X, t.Y, err)
		}
		_, err = s.Store.(DbStorage).Db.Exec(QUERY_INSERT_LOG, "cache", "insert cache", time.Now(), t.Z, t.X, t.Y)
		log.Printf("insert log db")
		if err != nil {
			log.Print(err)
		}
		log.Printf("cache de z:%d x:%d y:%d", t.Z, t.X, t.Y)
	default:
		log.Printf("Votre store est pas imp")
	}
	return nil
}

func (s *Storage) Get(t *Tile) error {
	switch s.Store.(type) {
	case DbStorage:
		type row struct {
			data []byte
			dthr time.Time
		}
		// faire une func getfromdb
		// voir http://golang.org/ref/spec#Type_assertions
		rows, err := s.Store.(DbStorage).Db.Query(QUERY_TILE_EXIST, t.Z, t.X, t.Y)
		if err != nil {
			log.Print(err)
			return err
		}
		defer rows.Close()

		for rows.Next() {
			var r row
			if err := rows.Scan(&r.data, &r.dthr); err != nil {
				log.Print(err)
			}
			if r.data != nil {
				log.Printf("hit z:%d x:%d y:%d from storage:%s", t.Z, t.X, t.Y, s.Name)
				t.Data = r.data
				t.Dthr = r.dthr
				go func() {
					_, err := s.Store.(DbStorage).Db.Exec(QUERY_INSERT_LOG, "hitcache", "hit cache", time.Now().UTC(), t.Z, t.X, t.Y)
					log.Printf("insert log db")
					if err != nil {
						log.Print(err)
					}
				}()

				// log.Printf("ts:%v", t.Dthr)

				if time.Since(t.Dthr).Hours() > float64(t.Ttl) {
					go func() {
						getTileFromOSM(s.Store.(DbStorage).Db, t)
						_, err = s.Store.(DbStorage).Db.Exec(QUERY_UPDATE_TILE_IN_CACHE, t.Data, time.Now().UTC(), t.Source.(SrcOSM).Urlwww, t.Z, t.X, t.Y)
						log.Printf("update de z:%d x:%d y:%d vielle de %s", t.Z, t.X, t.Y, time.Since(t.Dthr).String())
						if err != nil {
							log.Print(err)
						}
					}()
				}

				return nil
			}
		}
		if err := rows.Err(); err != nil {
			return err
		}
	case BetaStorage:
		return fmt.Errorf("betastorage not implemented")
	default:
		return fmt.Errorf("pas de storage pour toi ...")
	}

	// humm, si tes là c'est qu'il n'est pas dans le cache alors passons à WWW
	// switch sur les type de sources

	switch t.Source.(type) {
	case SrcOSM:
		log.Printf("get z:%d x:%d y:%d depuis %s via %s", t.Z, t.X, t.Y, t.Source.(SrcOSM).Urlwww, t.Source.(SrcOSM).Httpproxy)
		err := getTileFromOSM(s.Store.(DbStorage).Db, t)
		if err != nil {
			return err
		} else {
			go func() {
				s.Put(t)
			}()
			return nil
		}
	case SrcFRASTER:
		log.Printf("franceraster n'est pas implémenté :(")
	default:
		log.Printf("Je ne trouve pas votre source de donnée pour peupler le cache")
	}

	return fmt.Errorf("pas de tuiles")
}

func getTileFromOSM(db *sql.DB, t *Tile) error {
	// utiliser le proxy http si il est config
	if t.Source.(SrcOSM).Httpproxy != "" {
		proxyUrl, _ := url.Parse(t.Source.(SrcOSM).Httpproxy)
		http.ProxyURL(proxyUrl)
		http.DefaultTransport = &http.Transport{Proxy: http.ProxyURL(proxyUrl)}
		log.Printf("use proxy %s for %s", t.Source.(SrcOSM).Httpproxy, t.Source.(SrcOSM).Urlwww)
	}
	resp, err := http.Get(t.Source.(SrcOSM).Urlwww)
	if err != nil {
		go func() {
			_, err := db.Exec(QUERY_INSERT_LOG, "404", "not found", time.Now(), t.Z, t.X, t.Y)
			log.Printf("insert log db")
			if err != nil {
				log.Print(err)
			}
		}()
		return fmt.Errorf("Trouve pas la tuile sur WWW via %s proxy:%s", t.Source.(SrcOSM).Urlwww, t.Source.(SrcOSM).Httpproxy)
	}

	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("Je ne comprend pas très bien le contenu de cette tuile qui vient de %s", t.Source.(SrcOSM).Urlwww)
	}

	log.Printf("hitwww z:%d x:%d y:%d from %s", t.Z, t.X, t.Y, t.Source.(SrcOSM).Urlwww)

	t.Data = body

	return nil
}
