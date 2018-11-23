package cache

type CacheUtil struct {
	KvsCache map[string]interface{}
}

// 从缓存读取
func (c *CacheUtil) Get(arg string) (interface{}, bool) {
	if result, ok := c.KvsCache[arg]; ok {
		return result, true
	} else {
		return "", false
	}
}

// 设置到缓存中
func (c *CacheUtil) Set(arg string, result interface{}) {
	c.KvsCache[arg] = result
}

// 清除所有缓存
func (c *CacheUtil) ClearCache() {
	c.KvsCache = make(map[string]interface{})
}
