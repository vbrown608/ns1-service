package main

import (
	"encoding/json"
	"io"
	"log"
	"net/http"
	"time"

	ns1 "gopkg.in/ns1/ns1-go.v2/rest"
	models "gopkg.in/ns1/ns1-go.v2/rest/model/dns"
)

type apiConfig struct {
	*http.ServeMux
	zonesService
	// recordService
	zoneStore
	// recordStore
}

type zonesService interface {
	Create(*models.Zone) (*http.Response, error)
	Update(*models.Zone) (*http.Response, error)
	Delete(string) (*http.Response, error)
}

type zoneStore interface {
}

func (c *apiConfig) routes() *http.ServeMux {
	mux := http.NewServeMux()
	mux.HandleFunc("/zones", c.handleZones())
	return mux
}

func (c *apiConfig) handleZones() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		dec := json.NewDecoder(r.Body)
		var zone models.Zone
		err := dec.Decode(&zone)
		if err != nil {
			log.Fatal(err)
		}

		var resp *http.Response
		switch r.Method {
		case http.MethodPut:
			resp, err = c.zonesService.Create(&zone)
			// And persist to postgres
		case http.MethodPost:
			resp, err = c.zonesService.Update(&zone)
		case http.MethodDelete:
			resp, err = c.zonesService.Delete(zone.Zone)
		default:
			// Respond with method not supported
			return
		}
		if err != nil {
			log.Fatal(err)
			// Respond with 500
		}
		io.Copy(w, resp.Body)
		// Set status of w to resp.Status
	}
}

// func (s *server) handleRecords() http.HandlerFunc {
// 	return func(w http.ResponseWriter, r *http.Request) {
// 		switch r.Method {
// 		case http.MethodPut:
// 		case http.MethodPost:
// 		case http.MethodDelete:
// 		default:
// 			// Method not supported
// 		}
// 	}
// }

func main() {
	ns1Client := ns1.NewClient(
		&http.Client{Timeout: time.Second * 10},
		ns1.SetAPIKey("TODO"),
	)
	conf := apiConfig{
		zonesService: ns1Client.Zones,
	}
	mux := conf.routes()
	log.Fatal(http.ListenAndServe(":8080", mux))
}
