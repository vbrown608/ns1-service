package main

import (
	"bytes"
	"encoding/json"
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

func TestMain(m *testing.M) {
	boltDB, err := bolt.Open(tempfile(), 0600, nil)
	if err != nil {
		log.Fatal(err)
	}
	defer os.Remove(boltDB.Path())
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

type mockZoneService struct{}

func (zs *mockZoneService) Create(z *models.Zone) (*http.Response, error) {
	if z.Zone == "newzone.com" {
		f, err := os.Open("fixtures/create-200.json")
		if err != nil {
			return nil, err
		}
		return &http.Response{
			StatusCode: 200,
			Body:       ioutil.NopCloser(bytes.NewBufferString("")),
			Header:     make(http.Header),
		}, json.NewDecoder(f).Decode(z)
	}
	return nil, nil
}

func (zs *mockZoneService) Get(string) (*models.Zone, *http.Response, error) {
	return &models.Zone{}, &http.Response{}, nil
}

func (zs *mockZoneService) Update(z *models.Zone) (*http.Response, error) {
	if z.Zone == "newzone.com" {
		f, err := os.Open("fixtures/update-200.json")
		if err != nil {
			return nil, err
		}
		return &http.Response{
			StatusCode: 200,
			Body:       ioutil.NopCloser(bytes.NewBufferString("")),
			Header:     make(http.Header),
		}, json.NewDecoder(f).Decode(z)
	}
	return nil, nil
}

func (zs *mockZoneService) Delete(string) (resp *http.Response, err error) {
	return &http.Response{
		StatusCode: 200,
		Body:       ioutil.NopCloser(bytes.NewBufferString("")),
		Header:     make(http.Header),
	}, nil
}

func TestCreateUpdateDeleteZone(t *testing.T) {
	// CREATE
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
	resp.Body.Close()
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

	// UPDATE
	jsonBody = `{"TTL":1337}`
	resp, err = http.Post(ts.URL+"/zones/newzone.com", "application/json", strings.NewReader(jsonBody))
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Errorf("POST to /zones failed with %d", resp.StatusCode)
	}
	body, err = ioutil.ReadAll(resp.Body)
	resp.Body.Close()
	if err != nil {
		t.Fatal(err)
	}
	dbZone, err = conf.db.GetZone("newzone.com")
	if err != nil {
		t.Fatal(err)
	}
	if dbZone.TTL != 1337 {
		log.Println(dbZone.TTL)
		t.Fatal("Failed to update zone")
	}
	if !strings.Contains(string(body), "dns1.p06.nsone.net") {
		t.Fatal("Failed to respond with DNS servers")
	}

	// DELETE
	client = &http.Client{}
	req, err = http.NewRequest(http.MethodDelete, ts.URL+"/zones/newzone.com", nil)
	if err != nil {
		t.Fatal(err)
	}
	resp, err = client.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("DELETE zone failed with failed with %d", resp.StatusCode)
	}
}

// tempfile returns a temporary file path.
func tempfile() string {
	f, err := ioutil.TempFile("", "bolt-")
	if err != nil {
		panic(err)
	}
	if err := f.Close(); err != nil {
		panic(err)
	}
	if err := os.Remove(f.Name()); err != nil {
		panic(err)
	}
	return f.Name()
}
