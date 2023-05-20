package main

import (
	"fmt"
	"log"
	"os"
	"strings"
	"text/template"

	"gopkg.in/yaml.v3"
)

const (
	leftDelim  = "<<"
	rightDelim = ">>"
)

// 解析YAML文件
func parseYAMLFile(filename string, funcs template.FuncMap) (any, error) {
	content, err := os.ReadFile(filename)
	if err != nil {
		return nil, err
	}

	//data, err := unmarshalYAML(content)
	//if err != nil {
	//	return nil, err
	//}

	// 使用Go模板解析YAML文件并应用自定义函数
	tmpl := template.New("yaml").Delims(leftDelim, rightDelim).Funcs(funcs)
	tmpl, err = tmpl.Parse(string(content))
	if err != nil {
		return nil, err
	}

	var result strings.Builder
	err = tmpl.Execute(&result, nil)
	if err != nil {
		return nil, err
	}

	parsedData, err := unmarshalYAML([]byte(result.String()))
	if err != nil {
		return nil, err
	}

	return parsedData, nil
}

// 递归解析YAML
func unmarshalYAML(content []byte) (any, error) {
	var data any
	err := yaml.Unmarshal(content, &data)
	if err != nil {
		return nil, err
	}

	return convertYAML(data), nil
}

// 转换YAML中的数据类型
func convertYAML(data any) any {
	switch v := data.(type) {
	case map[string]any:
		result := make(map[string]any)
		for key, value := range v {
			result[key] = convertYAML(value)
		}
		return result
	case []any:
		result := make([]any, len(v))
		for i, value := range v {
			result[i] = convertYAML(value)
		}
		return result
	default:
		return v
	}
}

var idMap = make(map[string]int)

// 自增函数，若id相同则自增，否则从initNum开始
// 使用闭包（而不是全局变量），实现id在不同文件/区块中的自增，而不是每次程序运行都共享一个id表
func autoAdd(id string, initNum int) int {
	if _, ok := idMap[id]; ok {
		idMap[id]++
	} else {
		idMap[id] = initNum
	}

	return idMap[id]
}

func main() {
	// 注册自定义函数
	funcs := template.FuncMap{
		"auto_add": autoAdd,
	}

	//const inputFile = "example.yaml"
	const (
		inputFile  = "after-override.yaml"
		outputFile = "output.yaml"
	)

	// 解析YAML文件并应用自定义函数
	data, err := parseYAMLFile(inputFile, funcs)
	if err != nil {
		fmt.Println("Error parsing YAML:", err)
		return
	}

	// 输出解析后的数据
	fmt.Printf("%#v\n", data)

	// 把解析后的数据转换回YAML格式
	//encoder := yaml.NewEncoder(os.Stdout)
	//encoder.SetIndent(2)
	//err = encoder.Encode(data)
	//if err != nil {
	//	fmt.Println("Error marshalling YAML:", err)
	//	return
	//}

	// 转换为YAML格式
	output, err := yaml.Marshal(data)
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
