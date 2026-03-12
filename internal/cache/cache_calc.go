package cache

import "time"

type CacheCalc struct {
	currCacheMaxCount      int
	cacheMaxCount          int
	cleanupInterval        time.Duration
	cached                 int64
	uncached               int64
	lastCalc               time.Time
	calcInterval           time.Duration
	cachePercentCollection []int8
	cacheFilled            int8
	expired                int64
}

func (c *CacheCalc) calc(cachePercent int8) bool {
	// Если прошло много (в 1000 больше интервала перебалансировки) времени - просто зачищаем кеш на всякий случай
	if time.Now().UnixNano() > c.expired {
		c.reset()
		return true
	}

	needRecreateList := false
	// Если пришло время посчитать
	if time.Since(c.lastCalc) > c.calcInterval {
		c.lastCalc = time.Now()
		// Вычисляем текуший процент попаданий
		currCachePercent := int8(c.cached * 100 / (c.cached + c.uncached))
		// Сдвигаем последние 9 значений процента попаданий, записываем текущее и вычисляем среднее
		var calcAvg int8
		for i := 9; i >= 0; i-- {
			if i != 0 {
				c.cachePercentCollection[i] = c.cachePercentCollection[i-1]
			} else {
				c.cachePercentCollection[0] = currCachePercent
			}

			if i == 9 {
				calcAvg = c.cachePercentCollection[9]
			} else {
				calcAvg = (calcAvg + c.cachePercentCollection[i]) / 2
			}
		}

		// Если массив последних 10 попаданий не заполнен - продолжаем заполнять
		if c.cacheFilled < 10 {
			c.cacheFilled++
		} else {
			// Если заполнен - производим перерасчет, чтобы определить - увеличивать кеш или уменьшать на 5 процентов
			// Если текущий от среднего за последние 10 измерений отличается больше, чем на процент - перебалансируем
			if currCachePercent-calcAvg < 1 {
				needRecreateList = true
				if currCachePercent < cachePercent {
					c.currCacheMaxCount = c.currCacheMaxCount * 105 / 100
					if c.currCacheMaxCount > c.cacheMaxCount {
						c.currCacheMaxCount = c.cacheMaxCount
					}
				} else {
					c.currCacheMaxCount = c.currCacheMaxCount * 95 / 100
				}
				c.cacheFilled = 0
				c.cached = 0
				c.uncached = 0
			}
		}
	}
	return needRecreateList
}

func (c *CacheCalc) calcHit() {
	c.cached++
}
func (c *CacheCalc) calcMiss() {
	c.uncached++
}

func (c *CacheCalc) reset() {
	c.cacheFilled = 0
	c.cached = 0
	c.uncached = 0
	c.expired = time.Now().Add(c.cleanupInterval * 1000).UnixNano()
}
