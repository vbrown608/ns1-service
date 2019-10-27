package main

import (
	"io/ioutil"
	"log"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/boltdb/bolt"
	models "gopkg.in/ns1/ns1-go.v2/rest/model/dns"
)

var ts *httptest.Server
var conf apiConfig

type mockZoneService struct {
}

func TestMain(m *testing.M) {
	log.SetFlags(log.LstdFlags | log.Lshortfile)
	// @TODO use a tempfile, remove it after test run
	boltDB, err := bolt.Open("./ns1_test.db", 0600, nil)
	if err != nil {
		log.Fatal(err)
	}
	defer boltDB.Close()
	db := database{boltDB}
	db.Init()

	conf = apiConfig{
		zonesService: &mockZoneService{},
		db:           db,
	}
	mux := conf.routes()
	ts = httptest.NewServer(mux)
	defer ts.Close()
	os.Exit(m.Run())
}

func (zs *mockZoneService) Create(z *models.Zone) (*http.Response, error) {
	if z.Zone == "newzone.com" {
		f, err := os.Open("fixtures/create-200.json")
		if err != nil {
			return nil, err
		}
		return &http.Response{
			StatusCode: 200,
			Body:       f,
			Header:     make(http.Header),
		}, nil
	}
	return nil, nil
}

func (zs *mockZoneService) Update(*models.Zone) (*http.Response, error) {
	return &http.Response{}, nil
}

func (zs *mockZoneService) Delete(string) (resp *http.Response, err error) { return }

func TestCreateZone(t *testing.T) {
	jsonBody := `{"zone":"newzone.com"}`
	client := &http.Client{}
	req, err := http.NewRequest(http.MethodPut, ts.URL+"/zones", strings.NewReader(jsonBody))
	if err != nil {
		t.Fatal(err)
	}
	resp, err := client.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("POST to /zones failed with %d", resp.StatusCode)
	}
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		t.Fatal(err)
	}

	dbZone, err := conf.db.GetZone("newzone.com")
	if err != nil {
		t.Fatal(err)
	}
	if dbZone.ID != "52051b2c9f782d58bb4df41b" {
		t.Fatal("Failed to persist zone")
	}
	if !strings.Contains(string(body), "dns1.p06.nsone.net") {
		t.Fatal("Failed to respond with DNS servers")
	}
	if !strings.Contains(string(body), "Set your domain's DNS") {
		t.Fatal("Failed to respond with configuration instructions")
	}
}

func TestUpdateZone(t *testing.T) {
	jsonBody := `{"zone":"newzone.com"}`
	resp, err := http.Post(ts.URL+"/zones", "application/json", strings.NewReader(jsonBody))
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("POST to /zones failed with %d", resp.StatusCode)
	}
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		t.Fatal(err)
	}
	// Expect zone with ID
	log.Println(string(body))
}
