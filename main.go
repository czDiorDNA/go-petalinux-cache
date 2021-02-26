package main

import (
	"io"
	"net/http"
	"os"
	"path"
	"strings"
	"time"

	"github.com/gocolly/colly"
	"github.com/gocolly/colly/extensions"
	"github.com/gocolly/redisstorage"
)

const Version = "rel-v2018.2"
const BaseURL = "http://petalinux.xilinx.com/sswreleases/" + Version + "/"

func GetRootDir() string {
	root, _ := os.Getwd()
	return path.Join(root, "downloads", Version)
}

func IsFile(file string) bool {
	f, e := os.Stat(file)
	if e != nil {
		return false
	}
	return !f.IsDir()
}

func IsDir(file string) bool {
	f, e := os.Stat(file)
	if e != nil {
		return false
	}
	return f.IsDir()
}

func main() {
	c := colly.NewCollector(func(collector *colly.Collector) {
		collector.Async = true
		collector.AllowURLRevisit = true
		collector.SetRequestTimeout(100 * time.Second)
		extensions.RandomUserAgent(collector)
	})

	storage := &redisstorage.Storage{
		Address:  "127.0.0.1:6379",
		Password: "",
		DB:       0,
		Prefix:   "petalinux-cache-" + Version,
	}

	c.SetStorage(storage)

	downC := c.Clone()
	c.OnHTML("#indexlist", func(e *colly.HTMLElement) {
		e.ForEach("tr", func(index int, child *colly.HTMLElement) {
			// 跳过Table头和返回上一级
			if index <= 1 {
				return
			}
			
			url := child.DOM.Find(".indexcolname>a").Text()
			url = e.Request.URL.String() + url
			size := child.DOM.Find(".indexcolsize").Text()
			size = strings.Trim(size, " ")
			// 代表是目录
			if size == "-" {
				e.Request.Visit(url)
			} else {
				downC.Visit(url)
			}
		})
	})
	c.OnError(func(resp *colly.Response, err error) {
		resp.Request.Visit(resp.Request.URL.String())
	})

	downC.OnResponse(func(r *colly.Response) {
		filePath := strings.Replace(r.Request.URL.String(), BaseURL, "", -1)
		filePath = strings.Replace(filePath, ":", "_", -1)
		filePath = path.Join(GetRootDir(), filePath)
		if !IsDir(path.Dir(filePath)) {
			os.MkdirAll(path.Dir(filePath), 0777)
		}

		res, err := http.Get(r.Request.URL.String())
		if err != nil {
			downC.Visit(r.Request.URL.String())
			return
		}
		f, err := os.Create(filePath)
		if err != nil {
			downC.Visit(r.Request.URL.String())
			return
		}
		io.Copy(f, res.Body)
	})

	c.Visit(BaseURL)

	c.Wait()
	downC.Wait()
}
