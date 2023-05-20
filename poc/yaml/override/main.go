package main

import (
	"fmt"
	"log"
	"os"

	"gopkg.in/yaml.v3"
)

type Config struct {
	MetaNodeServices Service `yaml:"meta_node_services"`
	MDSServices      Service `yaml:"mds_services"`
}

type Service struct {
	Config map[string]any `yaml:"config"`
	Deploy []Deploy       `yaml:"deploy"`
}

type Deploy struct {
	Host   string         `yaml:"host"`
	Config map[string]any `yaml:"config,omitempty"`
}

func main() {
	const (
		inputFile  = "input.yaml"
		outputFile = "output.yaml"
	)
	// 读取YAML文件内容
	content, err := os.ReadFile(inputFile)
	if err != nil {
		log.Fatal("Error reading YAML file:", err)
	}

	// 解析YAML
	var config Config
	err = yaml.Unmarshal(content, &config)
	if err != nil {
		log.Fatal("Error unmarshaling YAML:", err)
	}

	// 处理配置
	handleConfig(&config)

	// 把每个 service 的全局配置 Config 都置空
	config.MetaNodeServices.Config = nil
	config.MDSServices.Config = nil

	// 转换为YAML格式
	output, err := yaml.Marshal(config)
	if err != nil {
		log.Fatal("Error marshaling YAML:", err)
	}

	// 将结果写入文件
	err = os.WriteFile(outputFile, output, 0644)
	if err != nil {
		log.Fatal("Error writing YAML file:", err)
	}

	fmt.Println("YAML processing complete.")
}

// 处理配置
func handleConfig(config *Config) {
	// 合并全局配置到每个服务的局部配置
	mergeConfig(&config.MetaNodeServices)
	mergeConfig(&config.MDSServices)
}

// 合并配置
func mergeConfig(service *Service) {
	globalConfig := service.Config
	for i := range service.Deploy {
		deploy := &service.Deploy[i]
		localConfig := deploy.Config
		if localConfig == nil {
			localConfig = make(map[string]any)
			deploy.Config = localConfig
		}
		for key, value := range globalConfig {
			if _, ok := localConfig[key]; !ok {
				localConfig[key] = value
			}
		}
	}
}
