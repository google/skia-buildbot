package cache

import (
	"sync"

	"go.skia.org/infra/go/util"
	"go.skia.org/infra/golden/go/diff"
)

type DiffCache struct {
	diffStore diff.DiffStore
	cache     map[string]*diff.DiffMetrics
	mutex     sync.RWMutex
}

func (d *DiffCache) Put(d1, d2 string, dm *diff.DiffMetrics) {
	d.mutex.Lock()
	defer d.mutex.Unlock()
	d.cache[d1+d2] = dm
}

func (d *DiffCache) Get(id string) *diff.DiffMetrics {
	d.mutex.RLock()
	defer d.mutex.RUnlock()
	return d.cache[id]
}

func (d *DiffCache) Keep(keepIds util.StringSet) {
}

// type DiffCache struct {
// 	group *groupcache.Group
// 	diffStore diff.DiffStore
// }

// func NewDiffCache(diffStore diff.DiffStore) *DiffCache {
// 	return &DiffCache{
// 		diffStore: diffStore,
// 	}
// 	ret.initGroup()
// 	return
// }

// func (d *DiffCache) initGroup() {
// 	d.group = groupcache.NewGroup("diff", 1<<34, groupcache.GetterFunc(
// 		func(ctx groupcache.Context, key string, dest groupcache.Sink) error {
// 			log.Println("looking up", key)
// 			v, ok := Store[key]
// 			if !ok {
// 				return errors.New("color not found")
// 			}
// 			dest.SetBytes(v)
// 			return nil
// 		},
// 	))
// }

// // Simple groupcache example: https://github.com/golang/groupcache
// // Running 3 instances:
// // go run groupcache.go -addr=:8080 -pool=http://127.0.0.1:8080,http://127.0.0.1:8081,http://127.0.0.1:8082
// // go run groupcache.go -addr=:8081 -pool=http://127.0.0.1:8081,http://127.0.0.1:8080,http://127.0.0.1:8082
// // go run groupcache.go -addr=:8082 -pool=http://127.0.0.1:8082,http://127.0.0.1:8080,http://127.0.0.1:8081
// // Testing:
// // curl localhost:8080/color?name=red
// package main

// import (
// 	"errors"
// 	"flag"
// 	"log"
// 	"net/http"
// 	"strings"

// 	"github.com/golang/groupcache"
// )

// var Store = map[string][]byte{
// 	"red":   []byte("#FF0000"),
// 	"green": []byte("#00FF00"),
// 	"blue":  []byte("#0000FF"),
// }

// var Group =

// func main() {
// 	addr := flag.String("addr", ":8080", "server address")
// 	peers := flag.String("pool", "http://localhost:8080", "server pool list")
// 	flag.Parse()
// 	http.HandleFunc("/color", func(w http.ResponseWriter, r *http.Request) {
// 		color := r.FormValue("name")
// 		var b []byte
// 		err := Group.Get(nil, color, groupcache.AllocatingByteSliceSink(&b))
// 		if err != nil {
// 			http.Error(w, err.Error(), http.StatusNotFound)
// 			return
// 		}
// 		w.Write(b)
// 		w.Write([]byte{'\n'})
// 	})
// 	p := strings.Split(*peers, ",")
// 	pool := groupcache.NewHTTPPool(p[0])
// 	pool.Set(p...)
// 	http.ListenAndServe(*addr, nil)
// }
