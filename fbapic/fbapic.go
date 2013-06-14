// Provides cached FB API calls.
package fbapic

import (
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/daaku/go.fbapi"
)

type Stats interface {
	Inc(name string)
	Record(name string, value float64)
}

type ByteCache interface {
	Store(key string, value []byte, timeout time.Duration) error
	Get(key string) ([]byte, error)
}

// Configure a Cached API accessor instance. You'll typically define
// one per type of cached call. An instance can be shared across
// goroutines.
type Cache struct {
	ByteCache ByteCache     // storage implementation
	Stats     Stats         // stats implementation
	Prefix    string        // cache key prefix
	Timeout   time.Duration // per value timeout
	Client    fbapi.Client  // Facebook API Client
}

// Make cached Graph API request.
func (c *Cache) Do(result interface{}, method string, path string, values ...fbapi.Values) error {
	var key string
	if method == "GET" || method == "HEAD" {
		key = fmt.Sprintf("%s:%s:%s", c.Prefix, method, path)
	}

	var raw []byte
	var err error
	if key != "" {
		raw, err = c.ByteCache.Get(key)
		if err != nil {
			c.Stats.Inc("fbapic storage.Get error")
			c.Stats.Inc("fbapic storage.Get error " + c.Prefix)
			return fmt.Errorf("fbapic error in storage.Get: %s", err)
		}

		err = json.Unmarshal(raw, result)
		if err != nil {
			return fmt.Errorf(
				"Request for path %s with response %s failed with "+
					"json.Unmarshal error %s.", path, string(raw), err)
		}
	}

	if raw == nil {
		c.Stats.Inc("fbapic cache miss")
		c.Stats.Inc("fbapic cache miss " + c.Prefix)
		start := time.Now()
		err = c.Client.Do(result, method, path, values...)
		if err != nil {
			c.Stats.Inc("fbapic graph api error")
			c.Stats.Inc("fbapic graph api error " + c.Prefix)
			return err
		}
		taken := float64(time.Since(start).Nanoseconds())
		c.Stats.Record("fbapic graph api time", taken)
		c.Stats.Record("fbapic graph api time "+c.Prefix, taken)

		err = c.ByteCache.Store(key, raw, c.Timeout)
		if err != nil {
			log.Printf("fbapic error in cache.Set: %s", err)
		}
	} else {
		c.Stats.Inc("fbapic cache hit")
		c.Stats.Inc("fbapic cache hit " + c.Prefix)
	}
	return nil
}
