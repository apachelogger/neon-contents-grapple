package main

import "github.com/boltdb/bolt"

type Database struct {
	bolt *bolt.DB
}

func NewDatabase() *Database {
	db, err := bolt.Open("my.db", 0600, nil)
	if err != nil {
		panic(err)
	}
	return &Database{
		bolt: db,
	}
}

func (db *Database) GetKeys(bucket string) []string {
	var keys []string
	err := boltDb.View(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(bucket))
		err := b.ForEach(func(k, v []byte) error {
			keys = append(keys, string(k))
			return nil
		})
		return err
	})
	if err != nil {
		panic(err)
	}
	return keys
}
