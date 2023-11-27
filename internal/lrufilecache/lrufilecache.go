package lrufilecache

import (
	"crypto/sha512"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path"
	"strings"

	"github.com/Dmit1812/imgresizr/internal/config"
	"github.com/Dmit1812/imgresizr/pkg/lrucache"
)

var headerExtension = config.OFileCacheHeaderExtension

type LRUFileCache struct {
	basepath     string
	Log          Logger
	fcache       lrucache.Cache
	mcache       lrucache.Cache
	toDeleteChan chan string
}

type CacheItem struct {
	Headers http.Header `json:"headers"`
	Content []byte      `json:"-"`
}

type Logger interface {
	Debug(msg string)
	Info(msg string)
	Warn(msg string)
	Error(msg string)
}

type Cache interface {
	Set(key string, ci CacheItem) bool
	Get(key string) (CacheItem, bool)
	Clear()
}

func (c *LRUFileCache) OnDeleteFunc(key string, _ interface{}) {
	c.toDeleteChan <- key
}

func createPath(basepath string) {
	if _, err := os.Stat(basepath); os.IsNotExist(err) {
		os.MkdirAll(basepath, 0o755)
	}
}

func NewLRUFileCache(fcapacity, mcapacity int, basepath string, logger Logger) *LRUFileCache {
	createPath(basepath)
	c := &LRUFileCache{}
	c.Log = logger
	c.fcache = lrucache.NewCacheWithOnDelete(fcapacity, c.OnDeleteFunc)
	c.mcache = lrucache.NewCache(mcapacity)
	c.basepath = basepath
	// create channel to receive file deletions
	c.toDeleteChan = make(chan string, fcapacity+1)
	// start deletion monitor
	go func() {
		for name := range c.toDeleteChan {
			c.deleteFile(name, true)
		}
	}()
	// load cache from disk
	c.LoadAll()
	// setup a go routine to receive files to delete
	c.Log.Info(fmt.Sprintf("LRUFileCache started for path %s", basepath))
	return c
}

func (c *LRUFileCache) LoadAll() {
	// Loop through the files in the basepath directory and load them into the cache
	files, err := os.ReadDir(c.basepath)
	if err != nil {
		c.Log.Error(err.Error())
	}
	n := 0
	for _, f := range files {
		if !f.IsDir() && !strings.Contains(f.Name(), headerExtension) {
			content, err := c.loadFile(f.Name())
			if err == nil {
				c.mcache.Set(f.Name(), content)
				c.fcache.Set(f.Name(), "")
				n++
			} else {
				// cleanup incorrect cache
				c.deleteFile(f.Name(), true)
			}
		}
	}
	c.Log.Info(fmt.Sprintf("LRUFileCache loaded %d files from %s", n, c.basepath))
}

func (c *LRUFileCache) deleteFile(name string, silent bool) {
	// Delete a file from the disk
	c.Log.Debug(fmt.Sprintf("deleting file %s", name))
	filename := path.Join(c.basepath, name)
	filenameH := filename + "." + headerExtension

	err := os.Remove(filename)
	errH := os.Remove(filenameH)

	if err != nil && !silent {
		c.Log.Error(err.Error())
		return
	}

	if errH != nil && !silent {
		c.Log.Error(err.Error())
		return
	}
}

func (c *LRUFileCache) loadFile(name string) (CacheItem, error) {
	// Load an individual file and return it's content
	var ci CacheItem
	var err error
	var jsonData []byte

	filename := path.Join(c.basepath, name)
	filenameH := filename + "." + headerExtension

	ci.Content, err = os.ReadFile(filename)
	if err != nil {
		c.Log.Error(err.Error())
		return CacheItem{}, err
	}

	jsonData, _ = os.ReadFile(filenameH)

	err = json.Unmarshal(jsonData, &ci.Headers)
	if err != nil {
		c.Log.Error(err.Error())

		return CacheItem{}, err
	}
	return ci, nil
}

func (c *LRUFileCache) saveFile(name string, ci CacheItem) error {
	// Save an individual file onto disk
	filename := path.Join(c.basepath, name)
	filenameH := filename + "." + headerExtension

	jsonData, err := json.Marshal(ci.Headers)
	if err != nil {
		c.Log.Error(err.Error())
		return err
	}
	c.Log.Debug(fmt.Sprintf("saving file %s", name))

	err = os.WriteFile(filename, ci.Content, 0o600)
	if err != nil {
		c.Log.Error(err.Error())
		c.deleteFile(name, true)
		return err
	}

	_ = os.WriteFile(filenameH, jsonData, 0o600)
	return nil
}

func (c *LRUFileCache) calculateHash(text string) string {
	// Calculate the sha-512 hash of the text
	data := []byte(text)
	hash := sha512.Sum512(data)
	// Convert the result to hex
	result := hex.EncodeToString(hash[:])
	// return the result
	return result
}

func (c *LRUFileCache) Set(uri string, ci CacheItem) bool {
	key := c.calculateHash(uri)
	return c.setByHash(key, ci)
}

func (c *LRUFileCache) setByHash(key string, ci CacheItem) bool {
	mfound := c.mcache.Set(key, ci)
	ffound := c.fcache.Set(key, "")
	if !ffound {
		c.saveFile(key, ci)
	}
	return mfound || ffound
}

func (c *LRUFileCache) Get(uri string) (CacheItem, bool) {
	key := c.calculateHash(uri)
	return c.getByHash(key)
}

func (c *LRUFileCache) getByHash(key string) (CacheItem, bool) {
	// to ensure that use counts are updated we ask both caches
	v, mok := c.mcache.Get(key)
	_, fok := c.fcache.Get(key) //nolint:ifshort

	// if found in memory return content
	if mok {
		if ci, ok := v.(CacheItem); ok {
			return ci, true
		}
	}

	// if not found in memory load from file system
	if fok {
		b, err := c.loadFile(key)
		if err == nil {
			c.mcache.Set(key, b)
			return b, true
		}
	}

	return CacheItem{}, false
}

func (c *LRUFileCache) Clear() {
	c.mcache.Clear()
	c.fcache.Clear()
}
