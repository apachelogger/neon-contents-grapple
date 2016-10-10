package main

import (
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"runtime"
	"runtime/pprof"
	"strings"
	"time"

	"github.com/boltdb/bolt"
	"github.com/gin-gonic/gin"
	"github.com/ryanuber/go-glob"

	_ "net/http/pprof"
)

var memprofile = flag.String("memprofile", "", "write memory profile to this file")

var db = NewDatabase()
var boltDb = db.bolt

func updateContents() {
	fmt.Println("updating")
	NewContents("http://archive.neon.kde.org/user/dists/xenial/main/Contents-amd64.gz").Get()
	NewContents("http://archive.ubuntu.com/ubuntu/dists/xenial/Contents-amd64.gz").Get()
	runtime.GC() // Force a cleanup
}

func Find(archive string, pattern string) map[string][]string {
	m := make(map[string][]string)

	err := boltDb.View(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(archive))
		b.ForEach(func(path, v []byte) error {
			matched := glob.Glob(pattern, string(path))
			if matched {
				subBucket := b.Bucket(path)
				var packages []string
				subBucket.ForEach(func(pkg, v []byte) error {
					packages = append(packages, string(pkg))
					return nil
				})
				m[string(path)] = packages
			}
			return nil
		})

		return nil
	})
	if err != nil {
		panic(err)
	}

	return m
}

func stringInSlice(a string, list []string) bool {
	for _, b := range list {
		if b == a {
			return true
		}
	}
	return false
}

func v1_archives(c *gin.Context) {
	c.JSON(http.StatusOK, db.GetKeys("archives"))
}

func v1_find(c *gin.Context) {
	query := c.Query("q")
	archive := strings.TrimPrefix(c.Param("archive"), "/")
	// Security... only allow querying actual archives. Not arbitrary buckets.
	if !stringInSlice(archive, db.GetKeys("archives")) {
		c.JSON(http.StatusOK, make(map[string]string))
		return
	}
	fmt.Println(archive)
	c.JSON(http.StatusOK, Find(archive, query))
}

func main() {
	flag.Parse()

	fmt.Printf("Hello, world.\n")

	err := boltDb.Update(func(tx *bolt.Tx) error {
		_, err := tx.CreateBucketIfNotExists([]byte("archives"))
		return err
	})
	if err != nil {
		panic(err)
	}

	go func() {
		log.Println(http.ListenAndServe("localhost:6060", nil))
	}()

	updateContents()
	updateTicker := time.NewTicker(30 * time.Minute)
	go func() {
		for {
			<-updateTicker.C
			updateContents()
		}
	}()

	if *memprofile != "" {
		f, err := os.Create(*memprofile)
		if err != nil {
			log.Fatal(err)
		}
		pprof.WriteHeapProfile(f)
		f.Close()
		return
	}

	fmt.Println("Ready to rumble...")

	router := gin.Default()
	router.GET("/v1/archives", v1_archives)
	router.GET("/v1/find/*archive", v1_find)
	router.Run()
}
