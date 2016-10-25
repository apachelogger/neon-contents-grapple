/*
	Copyright 2016 Harald Sitter <sitter@kde.org>

	This program is free software; you can redistribute it and/or
	modify it under the terms of the GNU General Public License as
	published by the Free Software Foundation; either version 3 of
	the License or any later version accepted by the membership of
	KDE e.V. (or its successor approved by the membership of KDE
	e.V.), which shall act as a proxy defined in Section 14 of
	version 3 of the license.

	This program is distributed in the hope that it will be useful,
	but WITHOUT ANY WARRANTY; without even the implied warranty of
	MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
	GNU General Public License for more details.

	You should have received a copy of the GNU General Public License
	along with this program.  If not, see <http://www.gnu.org/licenses/>.
*/

package main

import (
	"bufio"
	"compress/gzip"
	"io"
	"io/ioutil"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/boltdb/bolt"
)

type Contents struct {
	uri string
	id  string
}

func NewContents(uri string) *Contents {
	contents := &Contents{
		uri: uri,
	}
	contents.id = contents.getID()
	return contents
}

func (contents *Contents) Get() error {
	lastDate := []byte(nil)
	err := boltDb.View(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte("archives"))
		lastDate = b.Get([]byte(contents.id))
		return nil
	})
	if err != nil {
		panic(err)
	}

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

	// Writer the body to file
	// Reading from Body.Resp via Gzip and Bufio is substantially slower
	// than first downloading the entire body and reading from local. I am
	// not entirely sure why that is since bufio should make it fast :(
	tmpfile, err := ioutil.TempFile("", "neon-contents-grapple")
	if err != nil {
		return err
	}
	defer os.Remove(tmpfile.Name()) // clean up
	defer tmpfile.Close()
	_, err = io.Copy(tmpfile, resp.Body)
	if err != nil {
		return err
	}
	tmpfile.Seek(0, 0)

	gzip, err := gzip.NewReader(bufio.NewReader(tmpfile))
	if err != nil {
		panic(err)
	}
	defer gzip.Close()

	contents.process(bufio.NewReaderSize(gzip, 64*1024*1024), contents.id)

	err = boltDb.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte("archives"))
		b.Put([]byte(contents.id), []byte(resp.Header.Get("Date")))
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

func (contents *Contents) processLine(line string) {
	file, location := contents.parseLine(line)

	err := boltDb.Batch(func(tx *bolt.Tx) error {
		bucket := tx.Bucket([]byte(contents.id))
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

func (contents *Contents) findStart(reader *bufio.Reader) bool {
	foundStart := false
	var line string
	var err error
	for !foundStart {
		line, err = reader.ReadString('\n')
		if err == io.EOF {
			break
		} else if err != nil {
			panic(err)
		}
		foundStart = strings.HasPrefix(line, "FILE") && strings.Contains(line, "LOCATION")
	}
	return foundStart
}

func (contents *Contents) process(reader *bufio.Reader, hash string) {
	err := boltDb.Update(func(tx *bolt.Tx) error {
		_, err := tx.CreateBucketIfNotExists([]byte(hash))
		return err
	})
	if err != nil {
		panic(err)
	}

	if !contents.findStart(reader) {
		return // file seems invalid
	}

	input := make(chan string)

	go func() {
		var line string
		var err error
		for {
			line, err = reader.ReadString('\n')
			if err == nil {
				input <- line
				continue
			} else if err == io.EOF {
				break
			}
			panic(err)
		}
		close(input)
	}()

	var processorWg sync.WaitGroup
	for i := 0; i < 2048; i++ {
		// fmt.Println("creating worker ", strconv.Itoa(i))
		processorWg.Add(1)
		go contents.readLine(input, &processorWg, i)
	}

	processorWg.Wait()

	boltDb.Sync()
}

func (contents *Contents) readLine(input chan string, wg *sync.WaitGroup, i int) {
	defer wg.Done()
	for line := range input {
		// fmt.Println("worker " + strconv.Itoa(i) + " " + line)
		contents.processLine(line)
	}
}

func (contents *Contents) getID() string {
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
