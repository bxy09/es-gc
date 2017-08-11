package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"github.com/golang/glog"
	"io/ioutil"
	"net/http"
	"net/url"
	"reflect"
	"regexp"
	"sync"
	"time"
)

func ErrDetail(err error) string {
	if err == nil {
		return ""
	}
	return fmt.Sprintf("%s(%s)", err.Error(), reflect.TypeOf(err).String())
}

func main() {
	days := flag.Uint("d", 7, "reserve days")
	match := flag.String("m", ".*", "handle on matched indices (regexp)")
	urlStr := flag.String("url", "http://127.0.0.1:9200", "base url for the elastic search")
	timePattern := flag.String("time-pattern", `\d{4}\.\d{2}\.\d{2}$`, "time find pattern")
	timeLayout := flag.String("time-layout", `2006.01.02`, "time layout for golang:time.Parse()")

	flag.Parse()
	reg, err := regexp.Compile(*match)
	if err != nil {
		glog.Fatalf("Failed to parse the regexp:%s err:%s", *match, ErrDetail(err))
	}
	if *days <= 0 {
		glog.Fatal("reverse days num need be larger than zero!")
	}
	timeReg, err := regexp.Compile(*timePattern)
	if err != nil {
		glog.Fatalf("Failed to parse the time-pattern:%s err:%s", *timePattern, ErrDetail(err))
	}
	baseURL, err := url.Parse(*urlStr)
	if err != nil {
		glog.Fatalf("Failed to parse the url:%s err:%s", *baseURL, ErrDetail(err))
	}
	client := http.Client{Timeout: time.Second * 5}
	// do the stuff
	statURL := *baseURL
	statURL.Path += "/_stats"
	resp, err := client.Get(statURL.String())
	if err != nil {
		glog.Fatalf("Failed to query the _stat:%s err:%s", statURL.String(), ErrDetail(err))
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		glog.Fatalf("Failed to query the _stat:%s status:%s", statURL.String(), resp.Status)
	}
	statBytes, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		glog.Fatalf("Failed to read the _stat:%s err:%s", statURL.String(), ErrDetail(err))
	}
	stat := struct {
		Indices map[string]struct{} `json:"indices"`
	}{}
	err = json.Unmarshal(statBytes, &stat)
	if err != nil {
		glog.Fatalf("Failed to parse the _stat:%s err:%s", statURL.String(), ErrDetail(err))
	}
	indicis := stat.Indices
	// get max date
	found := []DateKey{}
	maxDate := time.Unix(0, 0)
	for key := range indicis {
		if !reg.MatchString(key) {
			continue
		}
		if timeStr := timeReg.FindString(key); len(timeStr) > 0 {
			date, err := time.Parse(*timeLayout, timeStr)
			if err != nil {
				glog.Errorf("Failed parse the time in the index key:%s err:%s", key, ErrDetail(err))
			}
			found = append(found, DateKey{date: date, key: key})
			if date.After(maxDate) {
				maxDate = date
			}
		}
	}
	wg := sync.WaitGroup{}
	filter := maxDate.AddDate(0, 0, -int(*days))
	for _, dk := range found {
		if dk.date.Before(filter) {
			wg.Add(1)
			go func(dk DateKey) {
				defer wg.Done()
				deleteURL := *baseURL
				deleteURL.Path += "/" + dk.key
				request, err := http.NewRequest(http.MethodDelete, deleteURL.String(), nil)
				if err != nil {
					glog.Errorf("Failed to generate request to delete the index, err:%s", ErrDetail(err))
				}
				resp, err := client.Do(request)
				if err != nil {
					glog.Errorf("Failed to do the request to delete the index:%s err:%s", dk.key, ErrDetail(err))
				}
				if resp.StatusCode != http.StatusOK {
					glog.Errorf("Failed to do the request to delete the index:%s err:%s", dk.key, resp.Status)
				}
				glog.Warning("Index Deleted :", dk.key)
			}(dk)
		}
	}
	wg.Wait()
}

type DateKey struct {
	date time.Time
	key  string
}
