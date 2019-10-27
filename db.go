package main

import (
	"encoding/json"

	"github.com/boltdb/bolt"
	models "gopkg.in/ns1/ns1-go.v2/rest/model/dns"
)

type database struct {
	*bolt.DB
}

var zonesBucket = []byte("zones")

func (db *database) Init() error {
	return db.Update(func(tx *bolt.Tx) error {
		_, err := tx.CreateBucketIfNotExists(zonesBucket)
		return err
	})
}

func (db *database) PutZone(z models.Zone) error {
	zb, err := json.Marshal(z)
	if err != nil {
		return err
	}
	return db.Update(func(tx *bolt.Tx) error {
		return tx.Bucket(zonesBucket).Put([]byte(z.Zone), zb)
	})
}

func (db *database) GetZone(zName string) (models.Zone, error) {
	var data []byte
	var z models.Zone
	err := db.View(func(tx *bolt.Tx) error {
		data = tx.Bucket(zonesBucket).Get([]byte(zName))
		return nil
	})
	if err != nil {
		return z, err
	}
	err = json.Unmarshal(data, &z)
	return z, err
}

func (db *database) DeleteZone(zName string) error {
	return db.Update(func(tx *bolt.Tx) error {
		return tx.Bucket(zonesBucket).Delete([]byte(zName))
	})
}
