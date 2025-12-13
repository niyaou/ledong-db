package cache

import (
	"time"

	"github.com/allegro/bigcache/v3"
)

var Cache *bigcache.BigCache

func Init(maxSizeMB int, shards int, lifeWindow, cleanWindow time.Duration) error {
	config := bigcache.DefaultConfig(lifeWindow)
	config.MaxEntriesInWindow = maxSizeMB * 1024 * 1024 / 512
	config.HardMaxCacheSize = maxSizeMB
	config.Shards = shards
	config.CleanWindow = cleanWindow

	cache, err := bigcache.NewBigCache(config)
	if err != nil {
		return err
	}

	Cache = cache
	return nil
}

func Get(key string) ([]byte, error) {
	return Cache.Get(key)
}

func Set(key string, value []byte) error {
	return Cache.Set(key, value)
}

func Delete(key string) error {
	return Cache.Delete(key)
}
