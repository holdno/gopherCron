package config

import (
	"os"

	"github.com/BurntSushi/toml"
)

// LoadFrom 加载配置文件
func LoadFrom(filePath string, conf interface{}) {
	_, err := os.Stat(filePath)
	if err != nil {
		panic(err)
	}

	_, err = toml.DecodeFile(filePath, conf)
	if err != nil {
		panic(err)
	}
}
