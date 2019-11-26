package main

import (
	"gopkg.in/yaml.v2"
	"io/ioutil"
	"log"
)

// Config 日志文件信息结构体。
type Config struct {
	// 日志服务器存放日志目录。
	App string
	// 日志服务器地址。
	SendURL string
	// # 日志查询周期(秒/s)
	Duration int
	// 过滤 windows 事件 ID，为 “,” 号分割的字符串。
	WinEvtIds string
}

var cfg *Config

// initConfig 初始化配置信息句柄。
func initConfig() {
	ReloadConfig()
	log.Println("Config init")
}

// GetConfig 读取日志文件。
func GetConfig() *Config {
	if cfg == nil {
		t := &Config{}

		buf, err := ioutil.ReadFile("config.yml")
		if err != nil {
			log.Fatalf("read config.yml error: %s", err)
		}
		err = yaml.Unmarshal(buf, t)
		if err != nil {
			log.Fatalf("config.yml file error: %s", err)
		}
		cfg = t
	}
	return cfg
}

// ReloadConfig --
func ReloadConfig() {
	t := &Config{}
	buf, err := ioutil.ReadFile("config.yml")
	if err != nil {
		log.Fatalf("read config.yml error: %s", err)
	}
	err = yaml.Unmarshal(buf, t)
	if err != nil {
		log.Fatalf("config.yml file error: %s", err)
	}
	cfg = t
}
