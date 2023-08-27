package provider

import (
	"bytes"
  "database/sql"
	"encoding/json"
  "log"
	"net/http"
  "os"
  "strconv"
  "strings"
  "time"

	"github.com/mmcloughlin/geohash"
	"github.com/spf13/viper"
  _ "github.com/mattn/go-sqlite3"
)

const cacheCreate string = `
  CREATE TABLE IF NOT EXISTS ipgeo_cache (
    ip STRING NOT NULL PRIMARY KEY,
    payload STRING NOT NULL,
    time DATETIME NOT NULL
  );`

const cacheSel string = `
  SELECT payload FROM ipgeo_cache WHERE (
    ip = ? AND time > ?
  );`

const cacheIns string = `REPLACE INTO ipgeo_cache VALUES (?, ?, ?);`

var cacheDB *sql.DB = nil

func GetCache() (*sql.DB, error) {
  if cacheDB != nil {
    return cacheDB, nil
  }

  cacheFile := viper.GetString("cache")
  db, err := sql.Open("sqlite3", cacheFile)

  if err != nil {
    log.Fatal(err)
    return nil, err
  }

  if _, err := db.Exec(cacheCreate); err != nil {
    log.Fatal(err)
    return nil, err
  }

  cacheDB = db
  return db, nil
}

func CacheLookup (ip string) (string, error) {
  db, err := GetCache()
  if err != nil {
    return "", err
  }

  row := db.QueryRow(cacheSel, ip, time.Now().Add(-24 * time.Hour))

  var payload string
  if err := row.Scan(&payload); err != nil {
    return "", err
  }

  return payload, nil
}

func CacheInsert(ip string, payload string) (error) {
  db, err := GetCache()
  if err != nil {
    log.Fatal(err)
    return err
  }

  _, err = db.Exec(cacheIns, ip, payload, time.Now())
  return err
}

// server url
const ipgeoServer = "https://api.ipgeolocation.io/ipgeo"

// Geo contains all the geolocation data
type ipgeoPayload struct {
	// CountryCode of the prisoner
	CountryCode string `json:"country_code2"`
	// Latitude of the prisoner
	Latitude string `json:"latitude"`
	// Longitude of the prisoner
	Longitude string `json:"longitude"`
  // isp of the prisoner
  ISP string `json:"isp"`
}

// ipgeo is a provider
type ipgeo struct{}

// Check if ipgeo is a provider on compile-time
var _ Provider = (*ipgeo)(nil)

// Lookup takes an ip and returns the geohash if everthing went well.
func (f ipgeo) Lookup(ip string) (Payload, error) {
  var jstr []byte

  if payload, err := CacheLookup(ip); err == nil {
    jstr = []byte(payload)
  } else {
    keyfile := viper.GetString("keyfile")
    key, err := os.ReadFile(keyfile)
    if err != nil {
      log.Print(err)
      return Payload{}, err
    }
    keystr := strings.TrimSuffix(string(key), "\n")
    resp, err := http.Get(ipgeoServer + "?apiKey=" + keystr + "&ip=" + ip)
    if err != nil {
      log.Print(err)
      return Payload{}, err
    }
    reader := new(bytes.Buffer)
    _, err = reader.ReadFrom(resp.Body)
    if err != nil {
      log.Print(err)
      return Payload{}, err
    }
    jstr = reader.Bytes()
    CacheInsert(ip, string(jstr))
  }

	var data ipgeoPayload
  if err := json.Unmarshal(jstr, &data); err != nil {
    log.Print(string(jstr))
    log.Print(err)
		return Payload{}, err
	}

  lat, err := strconv.ParseFloat(data.Latitude, 64)
  if err != nil {
    log.Print(err)
    return Payload{}, err
  }

  lng, err := strconv.ParseFloat(data.Longitude, 64)
  if err != nil {
    log.Print(err)
    return Payload{}, err
  }

	return Payload{data.CountryCode, geohash.Encode(lat, lng), data.ISP}, nil
}
