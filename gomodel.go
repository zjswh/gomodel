package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"regexp"
	"strings"
)

var sqlAddr = flag.String("sql", "", "sql file path")
var targetAddr = flag.String("dir", "", "model produced path")

func main()  {
	flag.Parse()
	if *sqlAddr == "" || !checkFileIsExist(*sqlAddr){
		fmt.Println("sql路径不存在")
		return
	}

	if *targetAddr == "" {
		fmt.Println("请输入文件生成地址")
		return
	}
	bt, _ := os.ReadFile(*sqlAddr)
	arr := strings.Split(string(bt), ";")
	for _, v := range arr {
		if v != "" && v != "\n"  && v != "\r\n" {
			sqlName := regexpData(v, "CREATE TABLE `(.*?)` ")
			dir := *targetAddr
			dir = strings.TrimRight(dir, "/")
			filename := dir + "/" + Case2Camel(sqlName) + ".go"
			if !checkFileIsExist(filename) { //如果文件存在
				var result struct{
					Data string `json:"data"`
					Error string `json:"error"`
				}
				res, _ := Request("https://www.printlove.cn/api/sql2gorm", map[string]interface{}{
					"ddl" : v + ";",
				}, map[string]interface{}{}, "POST", "form")
				json.Unmarshal(res, &result)
				structName := regexpData(result.Data, "type (.*?) struct")
				funcTemplateStr := Temp()
				funcTemplateStr = strings.Replace(funcTemplateStr, "Template", Case2Camel(structName), -1)
				arr1 := strings.Split(result.Data, "\n")
				importIndex := 2
				hasImport := false
				hasTableNameFunc := false
				for k, c := range arr1 {
					if strings.Contains(c, "import (") {
						importIndex = k + 1
						hasImport = true
					}
					if strings.Contains(c, "TableName()") {
						hasTableNameFunc = true
					}
				}

				var insertArr []string
				if hasImport == true {
					insertArr = []string{"\"gorm.io/gorm\"", ""}
				} else {
					insertArr = []string{"import \"gorm.io/gorm\"", ""}
				}
				arr1 = append(arr1[:importIndex], append(insertArr, arr1[importIndex:]...)...)
				if hasTableNameFunc == false {
					insertArr = []string{fmt.Sprintf("func (m *%s) TableName() string {", Case2Camel(sqlName)), fmt.Sprintf("\treturn \"%s\"", sqlName), "}", ""}
					arr1 = append(arr1, insertArr...)
				}
				result.Data = strings.Join(arr1, "\n")
				err := ioutil.WriteFile(filename, []byte(result.Data + "\n" + funcTemplateStr), 0644)
				if err != nil {
					fmt.Println("model文件生成失败，原因:"+ err.Error())
				}
				fmt.Println("文件生成成功")
			} else {
				fmt.Println("文件已存在")
			}
		}
	}
}

func Temp()  string {
	return "func (m *Template) Create(Db *gorm.DB) error {\n    err := Db.Model(&m).Create(&m).Error\n    return err\n}\n\nfunc (m *Template) Update(Db *gorm.DB, field ...string) error {\n    sql := Db.Model(&m)\n    if len(field) > 0 {\n        sql = sql.Select(field)\n    }\n    err := sql.Where(\"id\", m.Id).Updates(m).Error\n    return err\n}\n\nfunc (m *Template) GetInfo(Db *gorm.DB) error {\n    sql := Db.Model(m).Where(\"id = ? \", m.Id)\n    err := sql.First(&m).Error\n    return err\n}\n"
}

func regexpData(str string, pattern string) string {
	reg2 := regexp.MustCompile(pattern)
	result2 := reg2.FindAllStringSubmatch(str, -1)
	return result2[0][1]
}

// 下划线写法转为驼峰写法
func Case2Camel(name string) string {
	name = strings.Replace(name, "_", " ", -1)
	name = strings.Title(name)
	return strings.Replace(name, " ", "", -1)
}

func checkFileIsExist(filename string) bool {
	if _, err := os.Stat(filename); os.IsNotExist(err) {
		return false
	}
	return true
}

func Request(url string, data map[string]interface{}, header map[string]interface{}, method string, stype string) (body []byte, err error) {
	url = strings.ReplaceAll(strings.ReplaceAll(strings.ReplaceAll(url, "\n", ""), " ", ""), "\r", "")
	param := []byte("")
	if stype == "json" {
		param, _ = json.Marshal(data)
		header["Content-Type"] = "application/json"
	} else {
		s := ""
		for k, v := range data {
			s += fmt.Sprintf("%s=%v&", k, v)
		}
		header["Content-Type"] = "application/x-www-form-urlencoded"
		param = []byte(s)
	}

	client := &http.Client{}
	req, err := http.NewRequest(method, url, bytes.NewReader(param))
	if err != nil {
		err = fmt.Errorf("new request fail: %s", err.Error())
		return
	}

	for k, v := range header {
		req.Header.Add(k, fmt.Sprintf("%s", v))
	}

	res, err := client.Do(req)
	if err != nil {
		err = fmt.Errorf("do request fail: %s", err.Error())
		return
	}

	defer res.Body.Close()
	body, err = ioutil.ReadAll(res.Body)
	if err != nil {
		err = fmt.Errorf("read res body fail: %s", err.Error())
		return
	}
	return
}
