package main

import (
	"encoding/json"
	"io"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/boltdb/bolt"
	"github.com/gorilla/mux"
	ns1 "gopkg.in/ns1/ns1-go.v2/rest"
	models "gopkg.in/ns1/ns1-go.v2/rest/model/dns"
)

type apiConfig struct {
	*http.ServeMux
	zonesService
	recordsService
	db database
}

type zonesService interface {
	Create(*models.Zone) (*http.Response, error)
	Get(string) (*models.Zone, *http.Response, error)
	Update(*models.Zone) (*http.Response, error)
	Delete(string) (*http.Response, error)
}

type recordsService interface {
	Create(*models.Record) (*http.Response, error)
	Update(*models.Record) (*http.Response, error)
	Delete(string) (*http.Response, error)
}

func (c *apiConfig) routes() *mux.Router {
	r := mux.NewRouter()
	r.HandleFunc("/zones", c.putZone()).Methods("PUT")
	r.HandleFunc("/zones/{zName}", c.updateZone()).Methods("POST")
	r.HandleFunc("/zones/{zName}", c.deleteZone()).Methods("DELETE")
	r.HandleFunc("/zones/{zName}/{dName}", c.putRecord()).Methods("PUT")
	return r
}

func (c *apiConfig) putZone() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		dec := json.NewDecoder(r.Body)
		var z models.Zone
		err := dec.Decode(&z)
		if err != nil {
			// @TODO write custom error messages here instead of responding with
			// internal error messages.
			// @TODO respond with JSON error messages.
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
		resp, err := c.zonesService.Create(&z)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			// Proxy error to client
			w.WriteHeader(resp.StatusCode)
			io.Copy(w, resp.Body)
			return
		}

		err = c.handleUpdatedZone(resp, w)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
	}
}

func (c *apiConfig) updateZone() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		dec := json.NewDecoder(r.Body)
		var z models.Zone
		err := dec.Decode(&z)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		zName := mux.Vars(r)["zName"]
		if z.Zone != "" && z.Zone != zName {
			http.Error(w, "Zone name doesn't match record", http.StatusInternalServerError)
			return
		}
		z.Zone = zName
		resp, err := c.zonesService.Update(&z)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			// Proxy error to client
			io.Copy(w, resp.Body)
			w.WriteHeader(resp.StatusCode)
			return
		}
		err = c.handleUpdatedZone(resp, w)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
	}
}

func (c *apiConfig) handleUpdatedZone(updated *http.Response, w http.ResponseWriter) error {
	// Update our zone with the values returned by NS1
	// That way our records will capture the id, defaults, etc.
	var z models.Zone
	dec := json.NewDecoder(updated.Body)
	err := dec.Decode(&z)
	if err != nil {
		return err
	}

	err = c.db.PutZone(z)
	if err != nil {
		return err
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
		return err
	}
	w.Header().Set("Content-Type", "application/json")
	return nil
}

func (c *apiConfig) deleteZone() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		zName := mux.Vars(r)["zName"]
		resp, err := c.zonesService.Delete(zName)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			// Proxy error to client
			w.WriteHeader(resp.StatusCode)
			io.Copy(w, resp.Body)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		io.Copy(w, resp.Body)
	}
}

func (c *apiConfig) putRecord() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		dec := json.NewDecoder(r.Body)
		var rec models.Record
		err := dec.Decode(&rec)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
		resp, err := c.recordsService.Create(&rec)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			// Proxy error to client
			w.WriteHeader(resp.StatusCode)
			io.Copy(w, resp.Body)
			return
		}
		err = c.syncZone(rec.Zone)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
		w.WriteHeader(http.StatusOK)
	}
}

func (c *apiConfig) syncZone(zName string) error {
	// Store records within Zone documents using NS1's `dns.ZoneRecord`
	z, _, err := c.zonesService.Get(zName)
	if err != nil {
		return err
	}

	err = c.db.PutZone(*z)
	if err != nil {
		return err
	}
	return nil
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
		ns1.SetAPIKey(os.Getenv("NS1_API_KEY")),
	)
	conf := apiConfig{
		zonesService: ns1Client.Zones,
		db:           db,
	}
	mux := conf.routes()
	log.Fatal(http.ListenAndServe(":8080", mux))
}
