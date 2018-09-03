package i18n

import (
	"fmt"
	"io/ioutil"

	"gopkg.in/yaml.v2"
)

type I18n struct {
	KvsCache map[string]interface{}
}

func (i *I18n) SetLang(lang string) {
	filePath := fmt.Sprintf("conf/locales/%s.yml", lang)
	in, err := ioutil.ReadFile(filePath)
	if err != nil {
		panic(err)
	}
	yaml.Unmarshal(in, &i.KvsCache)

}

func (i *I18n) Get(key string) interface{} {
	if data, ok := i.KvsCache[key]; ok {
		return data
	}
	return ""
}

func (i *I18n) ClearCache() {
	i.KvsCache = make(map[string]interface{})
}
