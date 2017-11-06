/*
 * Licensed Materials - Property of tenxcloud.com
 * (C) Copyright 2016 TenxCloud. All Rights Reserved.
 * 2016-09-18  @author liuyang
 */

package log

import (
	"io/ioutil"
	"net/http"
	"strings"
	"time"

	"github.com/golang/glog"
)

// ESResponse represents the data structure of logging data returned from elasticsearch
type ESResponse struct {
	ID       string      `json:"_id,omitempty"`
	Took     int         `json:"took,omitempty"`
	TimeOut  bool        `json:"time_out, omitempty"`
	Shards   ESShards    `json:"_shards, omitempty"`
	Hits     ESHits      `json:"hits"`
	Status   int         `json:"status,omitempty"`
	Error    interface{} `json:"error,omitempty"`
	Aggs     ESAggs      `json:"aggregations,omitempty"`
	ScrollId string      `json:"_scroll_id,omitempty"`
}

// ESAggs represents the structure of aggregations
type ESAggs struct {
	AllFileNameKey AllFileName `json:"all_file_name"`
}
type AllFileName struct {
	DocCntErrUpBound int        `json:"doc_count_error_upper_bound"`
	SumOtherDocCnt   int        `json:"sum_other_doc_count"`
	Buckets          []ESBucket `json:buckets`
}

type ESBucket struct {
	Key    string `json:"key"`
	DocCnt int    `json:"doc_count"`
}

// ESShards represents the structure of logging response _shards
type ESShards struct {
	Total      int `json:"total"`
	Successful int `json:"successful"`
	Failed     int `json:"failed"`
}

// ESHits represents the data structure of logging response hits
type ESHits struct {
	Total    int     `json:"total"`
	MaxScore float32 `json:"max_score,omitempty"`
	Hits     []ESHit `json:"hits"`
}

// ESHit represents logging .hits.hits[i]
type ESHit struct {
	Index  string      `json:"_index,omitempty"`
	Type   string      `json:"_type,omitempty"`
	ID     string      `json:"_id,omitempty"`
	Score  float32     `json:"_score,omitempty"`
	Source ESHitSource `json:"_source, omitempty"`
}

// ESHitSource represents data structure of .hits.hits._source
type ESHitSource struct {
	Log        string                 `json:"log,omitempty"`
	Kubernetes map[string]string      `json:"kubernetes,omitempty"` // corresponding to variable requiredFields
	Stream     string                 `json:"stream,omitempty"`
	Docker     map[string]interface{} `json:"docker,omitempty"`
	TimeNano   string                 `json:"time_nano,omitempty"`
	Tag        string                 `json:"tag,omitempty"`
	Timestamp  time.Time              `json:"@timestamp,omitempty"`
	FileName   string                 `json:"filename,omitempty"`
}

// httpRequest sends http request and returns response as byte[]
func (lc *LoggingClient) httpRequest(httpMethod, url, requestbody string) ([]byte, int, error) {
	var statusCode int
	method := "httpRequest"

	request, err := http.NewRequest(httpMethod, url, strings.NewReader(requestbody))
	if err != nil {
		glog.Errorln("SendRequest create new request failed, error:", err)
		return nil, statusCode, err
	}

	request.Header.Set("Authorization", "Bearer "+lc.Token)
	response, err := lc.httpClient.Do(request)
	if err != nil {
		glog.Errorln(method, "Request log data failed, error:", err)
		return nil, statusCode, err
	}

	defer response.Body.Close()

	statusCode = response.StatusCode

	body, err := ioutil.ReadAll(response.Body)
	if err != nil {
		glog.Errorln(method, "Read http response failed", err)
		return nil, statusCode, err
	}
	return body, statusCode, err
}
