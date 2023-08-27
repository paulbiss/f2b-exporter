package provider

import (
	"bytes"
	"encoding/json"
	"net/http"

	"github.com/mmcloughlin/geohash"
)

// server url
const freeGeoIPServer = "https://freegeoip.app/json/"

// Geo contains all the geolocation data
type freeGeoIPPayload struct {
	// CountryCode of the prisoner
	CountryCode string `json:"country_code"`
	// Latitude of the prisoner
	Latitude float64 `json:"latitude"`
	// Longitude of the prisoner
	Longitude float64 `json:"longitude"`
}

// freeGeoIP is a provider
type freeGeoIP struct{}

// Check if freeGeoIP is a provider on compile-time
var _ Provider = (*freeGeoIP)(nil)

// Lookup takes an ip and returns the geohash if everthing went well.
func (f freeGeoIP) Lookup(ip string) (Payload, error) {
	resp, err := http.Get(freeGeoIPServer + ip)
	if err != nil {
		return Payload{}, err
	}

	var (
		data   freeGeoIPPayload
		reader = new(bytes.Buffer)
	)
	_, err = reader.ReadFrom(resp.Body)
	if err != nil {
		return Payload{}, err
	}
	if err = json.Unmarshal(reader.Bytes(), &data); err != nil {
		return Payload{}, err
	}

	return Payload{data.CountryCode, geohash.Encode(data.Latitude, data.Longitude), ""}, nil
}
