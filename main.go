package main

import (
	"encoding/json"
	"io"
	"log"
	"net/http"
	"time"

	"github.com/boltdb/bolt"
	ns1 "gopkg.in/ns1/ns1-go.v2/rest"
	models "gopkg.in/ns1/ns1-go.v2/rest/model/dns"
)

type apiConfig struct {
	*http.ServeMux
	zonesService
	db database
}

type zonesService interface {
	Create(*models.Zone) (*http.Response, error)
	Update(*models.Zone) (*http.Response, error)
	Delete(string) (*http.Response, error)
}

type zoneStore interface {
	Create(*models.Zone) error
	Update(*models.Zone) error
	Delete(string) error
}

func (c *apiConfig) routes() *http.ServeMux {
	mux := http.NewServeMux()
	mux.HandleFunc("/zones", c.handleZones())
	return mux
}

func (c *apiConfig) handleZones() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		dec := json.NewDecoder(r.Body)
		var z models.Zone
		err := dec.Decode(&z)
		if err != nil {
			log.Fatal(err)
		}

		var resp *http.Response
		switch r.Method {
		case http.MethodPut:
			resp, err := c.zonesService.Create(&z)
			if err != nil {
				c.Error("Failed to create zone", w)
				return
			}
			// Update our zone with the values returned by NS1
			// That way our records will capture the id, defaults, etc. that they've
			// chosen.
			dec := json.NewDecoder(resp.Body)
			err = dec.Decode(&z)
			if err != nil {
				c.Error("Failed to create zone", w)
				return
			}

			err = c.db.PutZone(z)
			if err != nil {
				c.Error("Failed to create zone", w)
				return
			}

			instructions := struct {
				DNSServers []string `json:"dns_servers"`
				Message    string   `json:"message"`
			}{
				DNSServers: z.DNSServers,
				Message:    `Set your domain's DNS servers to the hosts listed here. Normally you will do this in your domain registrar's portal. If this zone is a subdomain, you can do this by subdelegating the subdomain using NS records in the parent zone's DNS.`,
			}
			out, err := json.Marshal(instructions)
			if err != nil {
				c.Error("Error formatting zone instructions", w)
				return
			}
			w.Write(out)
		case http.MethodPost:
			resp, err = c.zonesService.Update(&z)
			io.Copy(w, resp.Body)
		case http.MethodDelete:
			resp, err = c.zonesService.Delete(z.Zone)
			io.Copy(w, resp.Body)
		default:
			// Respond with method not supported
			return
		}
		if err != nil {
			log.Fatal(err)
			// Respond with 500
		}
		// Set status of w to resp.Status
	}
}

func (c *apiConfig) Error(msg string, w http.ResponseWriter) {

}

func (c *apiConfig) handleRecords() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodPut:
		case http.MethodPost:
		case http.MethodDelete:
		default:
			// Method not supported
		}
	}
}

func main() {
	boltDB, err := bolt.Open("./ns1.db", 0600, nil)
	if err != nil {
		log.Fatal(err)
	}
	defer boltDB.Close()
	db := database{boltDB}
	db.Init()

	ns1Client := ns1.NewClient(
		&http.Client{Timeout: time.Second * 10},
		ns1.SetAPIKey("TODO"),
	)
	conf := apiConfig{
		zonesService: ns1Client.Zones,
		db:           db,
	}
	mux := conf.routes()
	log.Fatal(http.ListenAndServe(":8080", mux))
}
