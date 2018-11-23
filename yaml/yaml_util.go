package yaml

import (
	"io/ioutil"

	yaml "gopkg.in/yaml.v2"
)

type YamlUtil struct {
	KvsCache map[string]map[string]string
}

func (c *YamlUtil) InitConfig(configPath string) {
	in, err := ioutil.ReadFile(configPath)
	if err != nil {
		panic(err)
	}
	yaml.Unmarshal(in, &c.KvsCache)
}

func (c *YamlUtil) Get(key string) map[string]string {
	if data, ok := c.KvsCache[key]; ok {
		return data
	}
	return nil
}

func (c *YamlUtil) ClearCache() {
	c.KvsCache = make(map[string]map[string]string)
}
