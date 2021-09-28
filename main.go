package main

import (
	"encoding/json"
	"flag"
	"fmt"
	nested "github.com/antonfisher/nested-logrus-formatter"
	"github.com/shiena/ansicolor"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/viper"
	"io/ioutil"
	"os"
	"strings"
	"sync"
)

var (
	paramsMap  = make(map[string]string)
	headersMap = map[string]string{
		"User-Agent":   "CloudApp/8.9.1 (com.bitqiu.pan; build:99; iOS 14.7.0) Alamofire/4.7.0",
		"Content-Type": "application/x-www-form-urlencoded",
	}
	apiHome    = "https://pan-api.bitqiu.com"
	size       = 0
	remotePath = ""
	wg         sync.WaitGroup
)

type Resp struct {
	Code    string   `json:"code"`
	Message string   `json:"message"`
	Data    RespData `json:"data"`
}
type RespData struct {
	CurrentPage int        `json:"currentPage"`
	PageSize    int        `json:"pageSize"`
	Data        []Resource `json:"data"`
}

type RespDownloadData struct {
	CurrentPage int      `json:"currentPage"`
	PageSize    int      `json:"pageSize"`
	Data        Resource `json:"data"`
}

// Resource 资源信息
type Resource struct {
	ResourceId      string `json:"resourceId"`
	ResourceType    int    `json:"resourceType"`
	Name            string `json:"name"`
	Size            int    `json:"size"`
	DirLevel        int    `json:"dirLevel"`
	DirType         int    `json:"dirType"`
	CreateUid       int    `json:"createUid"`
	CreateTime      string `json:"createTime"`
	SnapTime        string `json:"snapTime"`
	ViewTime        string `json:"viewTime"`
	Url             string `json:"url"`
	ViewOffsetMills string `json:"viewOffsetMills"`
	ResourceUid     string `json:"resourceUid"`
	Type            string `json:"type"`
	ExtName         string `json:"extName"`
}

func init() {
	// 改变默认的 Usage，flag包中的Usage 其实是一个函数类型。这里是覆盖默认函数实现，具体见后面Usage部分的分析
	flag.Usage = usage
	//InitLog 初始化日志
	log.SetFormatter(&nested.Formatter{
		HideKeys:        true,
		ShowFullLevel:   true,
		TimestampFormat: "2006-01-02 15:04:05.000",
		// FieldsOrder: []string{"component", "category"},
	})
	// then wrap the log output with it
	log.SetOutput(ansicolor.NewAnsiColorWriter(os.Stdout))
	log.SetLevel(log.DebugLevel)

	// 初始化配置
	viper.SetConfigName("conf")
	viper.SetConfigType("toml")
	viper.AddConfigPath(".")
	err := viper.ReadInConfig()
	if err != nil {
		log.Fatal("read config failed: %v", err)
	}
	size = viper.GetInt("file.size")
	remotePath = viper.GetString("file.remote_path")
	// 设置 headers 键值
	paramsMap["access_token"] = viper.GetString("validation.access_token")
	paramsMap["app_id"] = viper.GetString("validation.app_id")
	paramsMap["open_id"] = viper.GetString("validation.open_id")
	paramsMap["platform"] = viper.GetString("validation.platform")
	paramsMap["user_id"] = viper.GetString("validation.user_id")
}
func usage() {
	fmt.Fprintf(os.Stderr, `tiler version: tiler/v0.1.0
Usage: tiler [-h] [-c filename]
`)
	flag.PrintDefaults()
}

// 获取目录下的文件ids 最大二级
func getDirFileIds() map[string]Resource {
	fileIds := make(map[string]Resource)
	paramsMap["parent_id"] = viper.GetString("file.dir_ids")
	paramsMap["desc"] = "1"
	paramsMap["limit"] = "1000"
	paramsMap["model"] = "1"
	paramsMap["order"] = "updateTime"
	paramsMap["page"] = "1"
	url := apiHome + "/fs/dir/resources/v2"
	resp, err := HttpPost(url, paramsMap, nil, headersMap)
	if err != nil {
		log.Fatal(err)
	}
	content, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		panic(err)
	}
	var _resp Resp
	err = json.Unmarshal(content, &_resp)
	if err != nil {
		log.Fatal("format err:%s\n", err.Error())

	}
	for _, val := range _resp.Data.Data {
		if val.ExtName != "" && val.Size >= size {
			fileIds[val.ResourceId] = val
		} else {
			// 进入下一级再获取文件资源id
			paramsMap["parent_id"] = val.ResourceId
			resp, err := HttpPost(url, paramsMap, nil, headersMap)
			if err != nil {
				log.Fatal(err)
			}
			content, err := ioutil.ReadAll(resp.Body)
			if err != nil {
				panic(err)
			}
			var _resp Resp
			err = json.Unmarshal(content, &_resp)
			for _, _val := range _resp.Data.Data {
				if _val.ExtName != "" && _val.Size >= size {
					fileIds[_val.ResourceId] = _val
				}
			}
		}
	}
	return fileIds
}

// 批量获取下载地址
func getDownloadLink(fileIds map[string]Resource) map[string]string {
	var rIds = make(map[string]string)
	url := apiHome + "/fs/download/file/url"
	for k, _ := range fileIds {
		paramsMap["file_ids"] = k
		resp, _ := HttpPost(url, paramsMap, nil, headersMap)
		content, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			panic(err)
		}
		var _resp RespDownloadData
		err = json.Unmarshal(content, &_resp)
		rIds[_resp.Data.Url] = "./download/" + fileIds[k].Name
	}
	return rIds
}

func main() {
	fileIds := getDirFileIds()
	//fmt.Printf("%s",fileIds)
	//file := [] string{"49ccb4b46bb3458bbb938d51971292f0"}
	rIds := getDownloadLink(fileIds)
	for url, path := range rIds {
		sp := strings.Split(path, "/")
		fileName := sp[len(sp)-1]
		if Exists(remotePath + fileName) {
			log.Printf("文件已存在 PATH:%s", remotePath+fileName)
			continue
		}
		wg.Add(1)
		downloadFile(url, path, &wg, headersMap)
		fmt.Printf("移动 源：%s -> %s", path, remotePath+fileName)
		err := MoveFile(path, remotePath+fileName)
		if err != nil {
			panic(err)
		}
	}
	wg.Wait()
}
