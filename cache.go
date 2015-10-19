package ttlcache

import (
	"sync"
	"time"
)

// ExpireCallback is used as a callback on item expiration
type expireCallback func(key string, value interface{})

// Cache is a synchronized map of items that can auto-expire once stale
type Cache struct {
	mutex                  sync.Mutex
	ttl                    time.Duration
	items                  map[string]*item
	expireCallback         expireCallback
	priorityQueue          *priorityQueue
	expirationNotification chan bool
	expirationTime         time.Time
}

func (cache *Cache) getItem(key string) (*item, bool) {
	cache.mutex.Lock()
	defer cache.mutex.Unlock()

	item, exists := cache.items[key]
	if !exists || item.expired() {
		return nil, false
	}

	if item.ttl >= 0 && (item.ttl > 0 || cache.ttl > 0) {
		if cache.ttl > 0 && item.ttl == 0 {
			item.ttl = cache.ttl
		}

		item.touch()
		cache.priorityQueue.update(item)
		cache.expirationNotificationTrigger(item)
	}

	return item, exists
}

func (cache *Cache) startExpirationProcessing() {
	for {
		var sleepTime time.Duration
		cache.mutex.Lock()
		if cache.priorityQueue.Len() > 0 {
			if cache.ttl > 0 && time.Now().Add(cache.ttl).Before(cache.priorityQueue.items[0].expireAt) {
				sleepTime = cache.ttl
			} else {
				sleepTime = cache.priorityQueue.items[0].expireAt.Sub(time.Now())
			}
		} else if cache.ttl > 0 {
			sleepTime = cache.ttl
		} else {
			sleepTime = time.Duration(1 * time.Hour)
		}

		cache.expirationTime = time.Now().Add(sleepTime)
		cache.mutex.Unlock()

		select {
		case <-time.After(cache.expirationTime.Sub(time.Now())):
			if cache.priorityQueue.Len() == 0 {
				continue
			}

			cache.mutex.Lock()
			item := cache.priorityQueue.items[0]

			if !item.expired() {
				cache.mutex.Unlock()
				continue
			}

			cache.priorityQueue.remove(item)
			delete(cache.items, item.key)
			cache.mutex.Unlock()

			if cache.expireCallback != nil {
				cache.expireCallback(item.key, item.data)
			}
		case <-cache.expirationNotification:
			continue
		}
	}
}

func (cache *Cache) expirationNotificationTrigger(item *item) {
	if cache.expirationTime.After(time.Now().Add(item.ttl)) {
		cache.expirationNotification <- true
	}
}

// Check doesn't update the TTL when fetching the record
func (cache *Cache) Check(key string) (interface{}, bool) {
	cache.mutex.Lock()
	defer cache.mutex.Unlock()

	item, exists := cache.items[key]
	if !exists || item.expired() {
		return nil, false
	}
	return item.data, exists
}

// Set is a thread-safe way to add new items to the map
func (cache *Cache) Set(key string, data interface{}) {
	cache.SetWithTTL(key, data, ItemExpireWithGlobalTTL)
}

// SetWithTTL is a thread-safe way to add new items to the map with individual ttl
func (cache *Cache) SetWithTTL(key string, data interface{}, ttl time.Duration) {
	item, exists := cache.getItem(key)
	cache.mutex.Lock()
	defer cache.mutex.Unlock()

	if exists {
		item.data = data
		item.ttl = ttl
	} else {
		item = newItem(key, data, ttl)
		cache.items[key] = item
	}

	if item.ttl >= 0 && (item.ttl > 0 || cache.ttl > 0) {
		if cache.ttl > 0 && item.ttl == 0 {
			item.ttl = cache.ttl
		}

		item.touch()

		if exists {
			cache.priorityQueue.update(item)
		} else {
			cache.priorityQueue.push(item)
		}

		cache.expirationNotificationTrigger(item)
	}
}

// Get is a thread-safe way to lookup items
// Every lookup, also touches the item, hence extending it's life
func (cache *Cache) Get(key string) (interface{}, bool) {
	item, exists := cache.getItem(key)
	if exists {
		return item.data, true
	}
	return nil, false
}

func (cache *Cache) Remove(key string) bool {
	cache.mutex.Lock()
	defer cache.mutex.Unlock()

	object, exists := cache.items[key]
	if !exists {
		return false
	}

	delete(cache.items, object.key)
	cache.priorityQueue.remove(object)

	return true
}

// Count returns the number of items in the cache
func (cache *Cache) Count() int {
	cache.mutex.Lock()
	defer cache.mutex.Unlock()

	return len(cache.items)
}

func (cache *Cache) SetTTL(ttl time.Duration) {
	cache.mutex.Lock()
	defer cache.mutex.Unlock()

	cache.ttl = ttl
	cache.expirationNotification <- true
}

func (cache *Cache) SetExpirationCallback(callback expireCallback) {
	cache.expireCallback = callback
}

// NewCache is a helper to create instance of the Cache struct
func NewCache() *Cache {
	cache := &Cache{
		items:                  make(map[string]*item),
		priorityQueue:          newPriorityQueue(),
		expirationNotification: make(chan bool, 1),
		expirationTime:         time.Now(),
	}
	go cache.startExpirationProcessing()
	return cache
}
