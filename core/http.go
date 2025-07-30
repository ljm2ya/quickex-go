package core

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"net/url"
	"strings"

	"errors"
)

type RequestType struct {
	Method      string
	BaseURL     string
	Url         string
	query       url.Values
	form        url.Values
	headers     map[string]string
	Header      http.Header
	body        io.Reader
	fullURL     string
	secType     int    //BINANCE
	recvWindow  int    //BINANCE
	TimeOffset  int64  //BINANCE
	QueryString string //BINANCE
}

type params map[string]interface{}

func (r *RequestType) SetBody(body io.Reader) *RequestType {
	r.body = body

	return r
}
func (r *RequestType) GetBody() io.Reader {
	return r.body
}

// setParam set param with key/value to query string
func (r *RequestType) SetParam(key string, value interface{}) *RequestType {
	if r.query == nil {
		r.query = url.Values{}
	}
	r.query.Set(key, fmt.Sprintf("%v", value))
	return r
}

func (r *RequestType) GetParam() url.Values {
	return r.query
}

// setParams set params with key/values to query string
func (r *RequestType) SetParams(m params) *RequestType {
	for k, v := range m {
		r.SetParam(k, v)
	}
	return r
}

// setFormParam set param with key/value to RequestType form body
func (r *RequestType) SetFormParam(key string, value interface{}) *RequestType {
	if r.form == nil {
		r.form = url.Values{}
	}
	r.form.Set(key, fmt.Sprintf("%v", value))
	return r
}

func (r *RequestType) GetForm() url.Values {
	return r.form
}

// setFormParams set params with key/values to RequestType form body
func (r *RequestType) SetFormParams(m params) *RequestType {
	for k, v := range m {
		r.SetFormParam(k, v)
	}
	return r
}

func (r *RequestType) SetHeader(key, value string) *RequestType {
	if r.headers == nil {
		r.headers = map[string]string{key: value}
	} else {
		r.headers[key] = value
	}
	return r
}

func (r *RequestType) Validate() (err error) {
	if r.query == nil {
		r.query = url.Values{}
	}
	if r.form == nil {
		r.form = url.Values{}
	}
	return nil
}

func parseRequest(r *RequestType) (err error) {
	fullURL := fmt.Sprintf("%s%s", r.BaseURL, r.Url)
	// fmt.Println(r.BaseURL, fullURL)

	queryString := r.query.Encode()
	body := &bytes.Buffer{}
	bodyString := r.form.Encode()
	header := http.Header{}
	if r.Header != nil {
		header = r.Header.Clone()
	}
	if r.headers != nil {
		for prop, value := range r.headers {
			header.Add(prop, value)
		}
	}
	if r.QueryString != "" {
		queryString = fmt.Sprintf("%s&%s", queryString, r.QueryString)
	}

	if queryString != "" {
		fullURL = fmt.Sprintf("%s?%s", fullURL, queryString)
	}

	if bodyString != "" {
		body = bytes.NewBufferString(bodyString)
	}

	r.fullURL = fullURL
	r.Header = header
	r.body = body
	return nil
}

func Request(r *RequestType, result interface{}, resultError interface{}) error {
	parseRequest(r)
	client := &http.Client{}

	req, err := http.NewRequest(r.Method, r.fullURL, r.body)
	if err != nil {
		return errors.Join(err, ErrHttp)
	}

	req.Header = r.Header

	res, err := client.Do(req)
	if err != nil {
		return errors.Join(err, ErrApi)
	}

	defer res.Body.Close()
	Body, err := ioutil.ReadAll(res.Body)
	if err != nil {
		return errors.Join(err, ErrResponseRead)
	}
	// fmt.Println(string(Body[:]))
	// Upbit
	if strings.Contains(string(Body[:]), "Too many") {
		return ErrApiTooMany
	}
	//
	r.Header = res.Header

	err = json.Unmarshal(Body, resultError)

	if res.StatusCode >= http.StatusBadRequest {
		return ErrApiRequest
	}

	err = json.Unmarshal(Body, result)
	if err != nil {
		return errors.Join(err, ErrUnmarshal)
	}
	return nil
}
