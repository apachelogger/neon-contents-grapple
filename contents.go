package main

import (
	"bufio"
	"compress/gzip"
	"io"
	"net/http"
	"net/url"
	"path/filepath"
	"strings"
	"sync"

	"github.com/boltdb/bolt"
)

type Contents struct {
	uri string
}

func NewContents(uri string) *Contents {
	return &Contents{
		uri: uri,
	}
}

func (contents *Contents) Get() error {
	archive := contents.archive()
	lastDate := []byte(nil)
	err := boltDb.View(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte("archives"))
		lastDate = b.Get([]byte(archive))
		return nil
	})
	if err != nil {
		panic(err)
	}

	// lastDate, lastDateErr := db.HGet("archives", archive).Result()

	client := &http.Client{}
	req, err := http.NewRequest("GET", contents.uri, nil)
	if lastDate != nil {
		req.Header.Set("If-Modified-Since", string(lastDate))
	}
	if err != nil {
		panic(err)
	}
	resp, err := client.Do(req)
	if err != nil {
		panic(err)
	}
	defer resp.Body.Close()

	if resp.ContentLength == 0 {
		return nil
	}

	gzip, err := gzip.NewReader(resp.Body)
	if err != nil {
		panic(err)
	}
	defer gzip.Close()

	contents.process(bufio.NewReader(gzip), archive)

	err = boltDb.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte("archives"))
		b.Put([]byte(archive), []byte(resp.Header.Get("Date")))
		return nil
	})
	if err != nil {
		panic(err)
	}

	boltDb.Sync()
	return nil
}

func (contents *Contents) parseLine(line string) (string, string) {
	parts := strings.Split(line, " ")
	if !(len(parts) >= 2) {
		panic("invalid line: " + line)
	}
	location, parts := parts[len(parts)-1], parts[:len(parts)-1] // pop
	file := strings.Join(parts, " ")                             // join to retain spaces in path

	// Ditch / if there even is one
	parts = strings.Split(location, "/")
	location = parts[len(parts)-1]

	file = strings.TrimSpace(file)
	location = strings.TrimSpace(location)

	return file, location
}

func (contents *Contents) processLine(line string, archive string, wg *sync.WaitGroup, sem chan bool) {
	defer wg.Done()
	defer func() { <-sem }()

	file, location := contents.parseLine(line)
	// fmt.Println(archive)
	// fmt.Println(file, location)

	err := boltDb.Batch(func(tx *bolt.Tx) error {
		bucket := tx.Bucket([]byte(archive))
		subBucket, err := bucket.CreateBucketIfNotExists([]byte(file))
		if err != nil {
			return err
		}
		err = subBucket.Put([]byte(location), nil)
		return err
	})
	if err != nil {
		panic(err)
	}
}

func (contents *Contents) process(reader *bufio.Reader, hash string) {
	foundStart := false
	err := boltDb.Update(func(tx *bolt.Tx) error {
		_, err := tx.CreateBucketIfNotExists([]byte(hash))
		return err
	})
	if err != nil {
		panic(err)
	}
	var wg sync.WaitGroup
	// Semaphore concurrent lines processing
	// Assuming the longer lines are of the form
	//   /usr/share/locale/ro/LC_MESSAGES/plasma_applet_org.kde.plasma.quickshare.mo
	// they'd be 75 characters long, assuming each character is a byte we'd want
	// to fill about 64 MiB worth of lines.
	// NB: actual consumption will be much higher due to pending bolt Batches,
	//     GC and so forth.
	sem := make(chan bool, 64*1024*1024/75)
	for {
		line, err := reader.ReadString('\n')
		if err == io.EOF {
			break
		} else if err != nil {
			panic(err)
		}
		if !foundStart {
			foundStart = strings.HasPrefix(line, "FILE") && strings.Contains(line, "LOCATION")
			continue
		}
		wg.Add(1)
		sem <- true
		go contents.processLine(line, hash, &wg, sem)
	}
	wg.Wait()
	boltDb.Sync()
}

func (contents *Contents) archive() string {
	u, err := url.Parse(contents.uri)
	if err != nil {
		panic(err)
	}
	uHost := u.Host
	uPath := filepath.Dir(u.Path) // ../ of file
	if strings.Contains(uPath, "main") {
		uPath = filepath.Dir(uPath) // ../../ of file for aptly hack
	}
	return filepath.Join(uHost, uPath)
}
