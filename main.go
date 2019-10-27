package main

import (
	"encoding/json"
	"io"
	"log"
	"net/http"
	"time"

	"github.com/boltdb/bolt"
	"github.com/gorilla/mux"
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

func (c *apiConfig) routes() *mux.Router {
	r := mux.NewRouter()
	r.HandleFunc("/zones", c.putZone()).Methods("PUT")
	// r.HandleFunc("/zones/{zName}", c.updateZone()).Methods("POST")
	r.HandleFunc("/zones/{zName}", c.deleteZone()).Methods("DELETE")
	return r
}

func (c *apiConfig) putZone() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		dec := json.NewDecoder(r.Body)
		var z models.Zone
		err := dec.Decode(&z)
		if err != nil {
			// @TODO custom error messages.
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
		resp, err := c.zonesService.Create(&z)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		// @TODO check response status

		// Update our zone with the values returned by NS1
		// That way our records will capture the id, defaults, etc.
		dec = json.NewDecoder(resp.Body)
		err = dec.Decode(&z)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		err = c.db.PutZone(z)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		instructions := struct {
			DNSServers []string `json:"dns_servers"`
			Message    string   `json:"message"`
		}{
			DNSServers: z.DNSServers,
			Message:    `Set your domain's DNS servers to the hosts listed here. Normally you will do this in your domain registrar's portal. If this zone is a subdomain, you can do this by subdelegating the subdomain using NS records in the parent zone's DNS.`,
		}
		enc := json.NewEncoder(w)
		if err := enc.Encode(instructions); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
	}
}

func (c *apiConfig) deleteZone() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		zName := mux.Vars(r)["zName"]
		resp, err := c.zonesService.Delete(zName)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		if resp.StatusCode != http.StatusOK {
			// @TODO
			return
		}
		w.Header().Set("Content-Type", "application/json")
		io.Copy(w, resp.Body)
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
