package main

import (
	"log"
	"net/http"
	"strconv"
	"time"

	"github.com/alicebob/sqlittle"
	"github.com/glvr182/f2b-exporter/provider"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/spf13/pflag"
	"github.com/spf13/viper"
)

// prisoner is an ip that has been (temp) banned by f2b
type prisoner struct {
	// jail represents the jail f2b assigned
	jail string
	// ip is the prisoners ip
	ip string
	// timeofban indicates moment he was banned
	timeofban int
	// bantime indicates how long he will be banned (-1 = infinity)
	bantime int
	// country is the general location of the prisoner
	country string
	// geohash is a more accurate location of the prisoner
	geohash string
	// currentlyBanned indicates if the prisoner is currently banned
	currentlyBanned bool
  // the ISP associated with this prisoner
  isp string
}

var (
	geocount = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "f2b_banned_ip",
			Help: "Number of banned IPs per country / region",
		},
		[]string{"country", "geohash", "jail", "currently_banned", "isp"},
	)
)

func init() {
	prometheus.MustRegister(geocount)
}

func main() {
	pflag.StringP("port", "p", "9119", "port to use for the exporter (defaults to 9119)")
	pflag.StringP("database", "d", "/var/lib/fail2ban/fail2ban.sqlite3", "fail2ban sqlite database location")
	pflag.StringP("remote", "r", "freeGeoIP", "remote provider to use (defaults to freeGeoIP)")
	pflag.StringP("keyfile", "k", "", "keyfile for remote provider API key")
	pflag.StringP("cache", "c", "", "cache for IPs to avoid requerying provider")
	pflag.Parse()
	if err := viper.BindPFlags(pflag.CommandLine); err != nil {
		log.Fatal(err)
	}
	viper.SetEnvPrefix("F2B") // will be uppercased automatically
	viper.AutomaticEnv()
	log.Println("Starting exporter")
	go func() {
		for {
			if err := update(); err != nil {
				log.Fatal(err)
			}
			time.Sleep(time.Minute)
		}
	}()

	http.Handle("/metrics", promhttp.Handler())
	if err := http.ListenAndServe(":"+viper.GetString("port"), nil); err != nil {
		log.Fatal(err)
	}
}

// update refreshes the prometheus metrics
func update() error {
	db, err := sqlittle.Open(viper.GetString("database"))
	if err != nil {
		return err
	}
	defer db.Close()

	provider, err := provider.New(viper.GetString("remote"))
	if err != nil {
		return err
	}

	prisoners, err := jailed(db, provider)
	if err != nil {
		return err
	}

	geocount.Reset()
	for _, prisoner := range prisoners {
		geocount.With(
      prometheus.Labels{
        "country": prisoner.country,
        "geohash": prisoner.geohash,
        "jail": prisoner.jail,
        "currently_banned": strconv.FormatBool(prisoner.currentlyBanned),
        "isp": prisoner.isp,
      },
    ).Inc()
	}

	return nil
}

// jailed is a helper function which fetches all the prisoners in the given database
func jailed(db *sqlittle.DB, provider provider.Provider) ([]prisoner, error) {
	var (
		prisoners []prisoner
		p         prisoner
		err       error
	)

	if err := db.SelectDone("bans", func(r sqlittle.Row) bool {
		err = r.Scan(&p.jail, &p.ip, &p.timeofban, &p.bantime)
		if err != nil {
			return true
		}

		payload, err := provider.Lookup(p.ip)
		if err != nil {
			return true
		}
		p.country = payload.CountryCode
		p.geohash = payload.GeoHash
    p.isp = payload.ISP

		if int64(p.timeofban+p.bantime) > time.Now().Unix() || p.bantime < 0 {
			p.currentlyBanned = true
		}

		prisoners = append(prisoners, p)
		return false

	}, "jail", "ip", "timeofban", "bantime"); err != nil {
		log.Fatal(err)
	}

	if err != nil {
		return nil, err
	}

	return prisoners, nil
}
