package config

import (
	"log"
	"os"

	"github.com/spf13/viper"
)

type Config struct {
	Port                      string `mapstructure:"PORT"`
	WechatGroupRobotWebhook   string `mapstructure:"WECHAT_GROUP_ROBOT_WEBHOOK"`
	WechatMessageRobotWebhook string `mapstructure:"WECHAT_MESSAGE_ROBOT_WEBHOOK"`
	RobotRemindsMobilePhones  string `mapstructure:"ROBOT_REMINDS_MOBILE_PHONES"`
	SupportEndpoint           string `mapstructure:"SUPPORT_ENDPOINT"`
	SupportUsername           string `mapstructure:"SUPPORT_USERNAME"`
	SupportPassword           string `mapstructure:"SUPPORT_PASSWORD"`
	FeishuEndpoint            string `mapstructure:"FEISHU_ENDPOINT"`
	FeishuAppID               string `mapstructure:"FEISHU_APP_ID"`
	FeishuAppSecret           string `mapstructure:"FEISHU_APP_SECRET"`
	FeishuTableAppToken       string `mapstructure:"FEISHU_TABLE_APP_TOKEN"`
	FeishuTableID             string `mapstructure:"FEISHU_TABLE_ID"`
}

var GlobalConfig *Config

func getDefaultConfig() Config {
	return Config{
		Port:                     "8080",
		WechatGroupRobotWebhook:  "",
		RobotRemindsMobilePhones: "",
		SupportEndpoint:          "",
		SupportUsername:          "",
		SupportPassword:          "",
		FeishuEndpoint:           "https://open.feishu.cn",
		FeishuAppID:              "",
		FeishuAppSecret:          "",
		FeishuTableID:            "",
		FeishuTableAppToken:      "",
	}
}

func loadConfigFromFile(path string, conf *Config) {
	var err error
	_, err = os.Stat(path)
	if err == nil {
		fileViper := viper.New()
		fileViper.SetConfigFile(path)
		if err = fileViper.ReadInConfig(); err == nil {
			if err = fileViper.Unmarshal(conf); err == nil {
				log.Printf("Load config from %s success\n", path)
				return
			}
		}
	}
	if err != nil {
		log.Fatalf("Load config from %s failed: %s\n", path, err)
	}
}

func Setup(configPath string) {
	var conf = getDefaultConfig()
	loadConfigFromFile(configPath, &conf)
	GlobalConfig = &conf
	log.Printf("%+v\n", GlobalConfig)
}

func GetConf() Config {
	if GlobalConfig == nil {
		return getDefaultConfig()
	}
	return *GlobalConfig
}
