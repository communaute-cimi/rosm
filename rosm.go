package main

import (
	"database/sql"
	"encoding/json"
	"flag"
	"fmt"
	"github.com/communaute-cimi/rosm/cache"
	"github.com/communaute-cimi/rosm/utils"
	_ "github.com/lib/pq"
	"log"
	"math/rand"
	"net/http"
	"os"
	"regexp"
	"runtime"
	"strconv"
	"time"
)

type Db struct {
	User     string
	Password string
	Name     string
	Host     string
	Port     string
}

type Configuration struct {
	Db     Db
	Cpu    int
	Ttl    int
	Listen string
	Proxy  string
	Osmsrv []string
}

var configfile string

func newDb(config Configuration) (*sql.DB, error) {
	cnx := fmt.Sprintf("host=%s user=%s password=%s dbname=%s", config.Db.Host, config.Db.User, config.Db.Password, config.Db.Name)
	db, err := sql.Open("postgres", cnx)
	// http://golang.org/pkg/database/sql/#DB.SetMaxIdleConns
	// SetMaxIdleConns sets the maximum number of connections in the idle connection pool.
	// TODO:mettre dans le fichier de config
	db.SetMaxIdleConns(5)
	// http://golang.org/pkg/database/sql/#DB.SetMaxOpenConns
	// SetMaxOpenConns sets the maximum number of open connections to the database.
	// TODO:mettre dans le fichier de config
	db.SetMaxOpenConns(20)

	if err != nil {
		return nil, err
	}

	return db, nil
}

func newConfig(configfile string) (*Configuration, error) {
	// todo: return err
	config := new(Configuration)
	file, err := os.Open(configfile)
	defer file.Close()
	if err != nil {
		log.Print("error: i can't load configgile - ", err)
	}
	decoder := json.NewDecoder(file)
	errjs := decoder.Decode(config)
	if errjs != nil {
		log.Print("error: i can't load configfile - ", err)
		os.Exit(0)
		return nil, err
	}
	return config, nil
}

func logConfig(config Configuration) {
	log.Printf("----------")
	log.Printf("Nb cpu:%d", config.Cpu)
	log.Printf("Listen:%s", config.Listen)
	log.Printf("http proxy:%s", config.Proxy)
	log.Printf("TTL:%dJ", config.Ttl)
	log.Printf("Db host:%s", config.Db.Host)
	log.Printf("Db user:%s", config.Db.User)
	log.Printf("Db name:%s", config.Db.Name)
	log.Printf("OSM:")
	for s := range config.Osmsrv {
		log.Printf("\t - %s", config.Osmsrv[s])
	}
	log.Printf("----------")
}

func mainhandler(db *sql.DB, config Configuration) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer utils.CheckDB(db)
		t := new(cache.Tile)
		tchan := make(chan *cache.Tile)
		began := time.Now()
		path := r.URL.Path[1:]
		re := regexp.MustCompile("^([0-9]+)/([0-9]+)/([0-9]+).png$")
		tilecoord := re.FindStringSubmatch(path)
		storage := cache.Storage{"pgsql", cache.DbStorage{db}} // init du storage avec un type dbstorage

		if tilecoord != nil {
			// TODO: mettre du test sur les err de strconv
			z, _ := strconv.Atoi(tilecoord[1])
			x, _ := strconv.Atoi(tilecoord[2])
			y, _ := strconv.Atoi(tilecoord[3])

			t.Z = z
			t.X = x
			t.Y = y
			urlTile := fmt.Sprintf("http://%s/%d/%d/%d.png", getsrvosm(config), t.Z, t.X, t.Y)
			t.Source = cache.SrcOSM{urlTile, config.Proxy}

			go func() {
				err := storage.Get(t)
				if err != nil {
					log.Printf("err: %s", err)
					http.Error(w, err.Error(), 500)
				}
				tchan <- t
			}()

			if b := (<-tchan).Data; b != nil {
				fmt.Fprintf(w, "%s", b)
			} else {
				// j'pas sus peupler la data de la tuile ni depuis cache ni depuis www :(
				http.Error(w, "Fucking error, le binaire de la tuile est vide !", 500)
			}

			log.Printf("info z:%d x:%d y:%d render in %s", z, x, y, time.Since(began))
		} else {
			http.NotFound(w, r)
		}
	})
}

func getsrvosm(config Configuration) string {
	srvs := config.Osmsrv
	return srvs[rand.Intn(len(srvs))]
}

func init() {
	// ajouter un mode debug
	flag.StringVar(&configfile, "c", "/etc/tiled.json", "Fichier de configuration")
}

func main() {
	flag.Parse()

	pconfig, err := newConfig(configfile)
	config := *pconfig // mouai bof :)

	if err != nil {
		log.Fatal("Prob lors du load du fichier de config")
	}

	logConfig(config)

	// maxcpu que l'app peut utiliser
	runtime.GOMAXPROCS(config.Cpu)

	db, err := newDb(config)

	if err != nil {
		log.Fatal("Err cnx db : %s", err)
	}

	// http.HandleFunc("/static/", func(w http.ResponseWriter, r *http.Request) {
	// 	http.ServeFile(w, r, r.URL.Path[1:])
	// })

	// https://gist.github.com/tsenart/5fc18c659814c078378d
	//http.Handle("/dashboard/", dashhandler(db))

	http.Handle("/", mainhandler(db, config))
	// start srv
	http.ListenAndServe(":"+config.Listen, nil)
}
