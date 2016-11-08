/* Copyright (c) 2016 Jason Ish
 * All rights reserved.
 *
 * Redistribution and use in source and binary forms, with or without
 * modification, are permitted provided that the following conditions
 * are met:
 *
 * 1. Redistributions of source code must retain the above copyright
 *    notice, this list of conditions and the following disclaimer.
 * 2. Redistributions in binary form must reproduce the above copyright
 *    notice, this list of conditions and the following disclaimer in the
 *    documentation and/or other materials provided with the distribution.
 *
 * THIS SOFTWARE IS PROVIDED ``AS IS'' AND ANY EXPRESS OR IMPLIED
 * WARRANTIES, INCLUDING, BUT NOT LIMITED TO, THE IMPLIED WARRANTIES OF
 * MERCHANTABILITY AND FITNESS FOR A PARTICULAR PURPOSE ARE
 * DISCLAIMED. IN NO EVENT SHALL THE AUTHOR BE LIABLE FOR ANY DIRECT,
 * INDIRECT, INCIDENTAL, SPECIAL, EXEMPLARY, OR CONSEQUENTIAL DAMAGES
 * (INCLUDING, BUT NOT LIMITED TO, PROCUREMENT OF SUBSTITUTE GOODS OR
 * SERVICES; LOSS OF USE, DATA, OR PROFITS; OR BUSINESS INTERRUPTION)
 * HOWEVER CAUSED AND ON ANY THEORY OF LIABILITY, WHETHER IN CONTRACT,
 * STRICT LIABILITY, OR TORT (INCLUDING NEGLIGENCE OR OTHERWISE) ARISING
 * IN ANY WAY OUT OF THE USE OF THIS SOFTWARE, EVEN IF ADVISED OF THE
 * POSSIBILITY OF SUCH DAMAGE.
 */

package elasticsearch

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	eveboxhttp "github.com/jasonish/evebox/http"

	"github.com/GeertJohan/go.rice"
	"github.com/jasonish/evebox/log"
)

type ElasticSearch struct {
	baseUrl  string
	username string
	password string

	EventSearchIndex string
	EventBaseIndex   string

	// These are filled on a ping request. Perhaps when creating an
	// instance of this object we should ping.
	VersionString string
	MajorVersion  int64

	HttpClient *eveboxhttp.HttpClient
}

func New(url string) *ElasticSearch {
	HttpClient := eveboxhttp.NewHttpClient()
	HttpClient.SetBaseUrl(url)

	es := &ElasticSearch{
		baseUrl:    url,
		HttpClient: HttpClient,
	}

	return es
}

func (es *ElasticSearch) DisableCertCheck(disableCertCheck bool) {
	es.HttpClient.DisableCertCheck(disableCertCheck)
}

func (es *ElasticSearch) SetUsernamePassword(username ...string) error {
	return es.HttpClient.SetUsernamePassword(username...)
}

func (es *ElasticSearch) SetEventIndex(index string) {
	if strings.HasSuffix(index, "*") {
		baseIndex := strings.TrimSuffix(strings.TrimSuffix(index, "*"), "-")
		es.EventSearchIndex = index
		es.EventBaseIndex = baseIndex
	} else {
		es.EventBaseIndex = index
		es.EventSearchIndex = fmt.Sprintf("%s-*", index)
	}
	log.Info("Event base index: %s", es.EventBaseIndex)
	log.Info("Event search index: %s", es.EventSearchIndex)
}

func (es *ElasticSearch) Decode(response *http.Response, v interface{}) error {
	decoder := json.NewDecoder(response.Body)
	decoder.UseNumber()
	return decoder.Decode(v)
}

func (es *ElasticSearch) Ping() (*PingResponse, error) {

	response, err := es.HttpClient.Get("")
	if err != nil {
		return nil, err
	}

	if response.StatusCode != 200 {
		return nil, NewElasticSearchError(response)
	}

	var body PingResponse
	if err := es.Decode(response, &body); err != nil {
		return nil, err
	}
	return &body, nil
}

func (es *ElasticSearch) CheckTemplate(name string) (exists bool, err error) {
	response, err := es.HttpClient.Head(fmt.Sprintf("_template/%s", name))
	if err != nil {
		return exists, err
	}
	exists = response.StatusCode == 200
	return exists, nil
}

func (es *ElasticSearch) GetTemplate(name string) (JsonMap, error) {
	response, err := es.HttpClient.Get(fmt.Sprintf("_template/%s", name))
	if err != nil {
		return nil, err
	}

	template := JsonMap{}

	if err := es.Decode(response, &template); err != nil {
		return nil, err
	}

	return template, nil
}

// GetKeywordType is a crude way of determining if the template is using
// Logstash 5 keyword type, or Logstash 2 "raw" type.
func (es *ElasticSearch) GetKeywordType(index string) (string, error) {
	if index == "" {
		index = es.EventBaseIndex
	}
	template, err := es.GetTemplate(index)
	if err != nil {
		return "", err
	}

	dynamicTemplates := template.GetMap(index).
		GetMap("mappings").
		GetMap("_default_").
		GetMapList("dynamic_templates")
	for _, entry := range dynamicTemplates {
		if entry["string_fields"] != nil {
			mappingType := entry.GetMap("string_fields").
				GetMap("mapping").
				GetMap("fields").
				GetMap("keyword").
				Get("type")
			if mappingType == "keyword" {
				return "keyword", nil
			}
		}
	}
	return "raw", nil
}

func (es *ElasticSearch) LoadTemplate(index string, majorVersion int64) error {

	templateBox, err := rice.FindBox("static")
	if err != nil {
		return err
	}

	var templateFilename string

	if majorVersion == 5 {
		templateFilename = "template-es5x.json"
	} else if majorVersion == 2 {
		templateFilename = "template-es2x.json"
	} else {
		return fmt.Errorf("No template for Elastic Search with major version %d", majorVersion)
	}

	templateFile, err := templateBox.Open(templateFilename)
	if err != nil {
		return err
	}

	decoder := json.NewDecoder(templateFile)
	decoder.UseNumber()

	var template map[string]interface{}
	err = decoder.Decode(&template)
	if err != nil {
		return err
	}
	template["template"] = fmt.Sprintf("%s-*", index)

	response, err := es.HttpClient.PutJson(fmt.Sprintf("_template/%s", index), template)
	if err != nil {
		return err
	}
	if response.StatusCode != http.StatusOK {
		return NewElasticSearchError(response)
	}
	es.HttpClient.DiscardResponse(response)

	// Success.
	return nil
}

func (es *ElasticSearch) Search(query interface{}) (*SearchResponse, error) {
	path := fmt.Sprintf("%s/_search", es.EventSearchIndex)
	response, err := es.HttpClient.PostJson(path, query)
	if err != nil {
		return nil, err
	}
	result := SearchResponse{}
	if err := es.Decode(response, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

func (es *ElasticSearch) SearchScroll(body interface{}, duration string) (*SearchResponse, error) {
	path := fmt.Sprintf("%s/_search?scroll=%s", es.EventSearchIndex, duration)
	response, err := es.HttpClient.PostJson(path, body)
	if err != nil {
		return nil, err
	}
	if response.StatusCode != http.StatusOK {
		return nil, NewElasticSearchError(response)
	}
	searchResponse := SearchResponse{}
	if err := DecodeResponse(response.Body, &searchResponse); err != nil {
		return nil, err
	}
	return &searchResponse, nil
}

func (es *ElasticSearch) Scroll(scrollId string, duration string) (*SearchResponse, error) {
	body := m{
		"scroll_id": scrollId,
		"scroll":    duration,
	}
	response, err := es.HttpClient.PostJson("_search/scroll", body)
	if err != nil {
		return nil, err
	}
	searchResponse := SearchResponse{}
	if err := DecodeResponse(response.Body, &searchResponse); err != nil {
		return nil, err
	}
	return &searchResponse, nil
}

func (es *ElasticSearch) DeleteScroll(scrollId string) (*http.Response, error) {
	return es.HttpClient.Delete("_search/scroll", "application/json",
		strings.NewReader(scrollId))
}

func DecodeResponse(reader io.Reader, output interface{}) error {
	decoder := json.NewDecoder(reader)
	decoder.UseNumber()
	return decoder.Decode(output)
}
