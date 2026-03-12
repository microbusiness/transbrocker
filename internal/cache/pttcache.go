package cache

import (
	"sync"
	"time"
	"transbroker/internal/domain"
)

type Cache struct {
	sync.RWMutex

	cacheMaxCount   *int
	cleanupInterval time.Duration
	expireAt        time.Time

	transItems map[TransIdent]domain.PreparedData

	transCacheCalc CacheCalc
}

func NewCache(cacheMaxCount *int, cleanupInterval *int) *Cache {
	c := &Cache{
		transCacheCalc:  CacheCalc{cached: 0, uncached: 0},
		cacheMaxCount:   cacheMaxCount,
		cleanupInterval: time.Duration(*cleanupInterval) * time.Second,
	}
	go c.resetIfExpired()
	return c
}

func (c *Cache) SetTrans(key TransIdent, value domain.PreparedData) {
	c.Lock()
	defer c.Unlock()

	if c.transItems == nil {
		c.initTransMap()
	}

	if c.isTransMapReachMaxLen() {
		c.partialCleanTransMap()
	}
	c.transItems[key] = value
}

func (c *Cache) GetTrans(key TransIdent) (domain.PreparedData, bool) {
	c.RLock()
	defer c.RUnlock()

	if c.transItems == nil {
		return domain.PreparedData{}, false
	}

	result, ok := c.transItems[key]
	if ok {
		c.transCacheCalc.calcHit()
		return result, true
	}
	c.transCacheCalc.calcMiss()
	return domain.PreparedData{}, false
}

func (c *Cache) RemoveTrans(key TransIdent) {
	c.Lock()
	if c.transItems != nil {
		delete(c.transItems, key)
	}
	c.Unlock()
}

func (c *Cache) GetCacheHitStat() int8 {
	c.Lock()
	defer c.Unlock()

	var transCachePercent int8 = 0

	if c.transCacheCalc.cached+c.transCacheCalc.uncached != 0 {
		transCachePercent = int8(c.transCacheCalc.cached * 100 / (c.transCacheCalc.cached + c.transCacheCalc.uncached))
	}

	return transCachePercent
}

func (c *Cache) isTransMapReachMaxLen() bool {
	if c.cacheMaxCount == nil {
		return false
	}

	return len(c.transItems) >= *c.cacheMaxCount
}

func (c *Cache) partialCleanTransMap() {
	const (
		toCleanPercent    = 10
		minimalCntToClean = 10
	)

	toCleanCount := (len(c.transItems) / 100) * toCleanPercent
	if toCleanCount < minimalCntToClean {
		toCleanCount = minimalCntToClean
	}

	counter := 0
	for k := range c.transItems {
		delete(c.transItems, k)
		counter++
		if counter == toCleanCount {
			break
		}
	}
}

func (c *Cache) resetIfExpired() {
	if c.cleanupInterval == 0 {
		return
	}

	<-time.After(c.cleanupInterval)
	c.Lock()
	c.initTransMap()
	c.Unlock()

	go c.resetIfExpired()
}

func (c *Cache) initTransMap() {
	if c.cacheMaxCount == nil {
		c.transItems = make(map[TransIdent]domain.PreparedData)
	} else {
		c.transItems = make(map[TransIdent]domain.PreparedData, *c.cacheMaxCount)
	}
}
