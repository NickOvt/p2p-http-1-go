package main

import (
	"encoding/json"
	"strconv"
	"strings"
)

type HTTPRequest struct {
	version     string
	headers     map[string]string
	requestType string
	path        string
	data        string // json
}

func (req *HTTPRequest) build() string {
	// add checks for valid data

	httpString := req.requestType + " " + req.path + " HTTP/" + req.version + CRLF

	if req.headers == nil {
		// if headers nil create empty map
		req.headers = map[string]string{}
	}

	_, isConnectionTypeInHeaders := req.headers["Connection"]

	if !isConnectionTypeInHeaders {
		// connection not specified, check x-own-ip, if present keep-alive, otherwise default to close
		_, isXOwnIpInHeaders := req.headers["X-Own-IP"]
		if isXOwnIpInHeaders {
			req.headers["Connection"] = "keep-alive"
		} else {
			req.headers["Connection"] = "close"
		}
	}

	if req.requestType == REQ_POST || req.requestType == REQ_PUT && len(req.data) > 0 {
		// POST or PUT request. Add content length header calculated from data size
		req.headers["Content-Length"] = strconv.Itoa(len(req.data))
	}

	for headerKey, headerValue := range req.headers {
		httpString += headerKey + ":" + headerValue + CRLF
	}

	httpString += CRLF // headers end with CRLFCRLF

	if req.requestType == REQ_POST || req.requestType == REQ_PUT && len(req.data) > 0 {
		// in POST and PUT requests add data after headers
		httpString += string(req.data)
	}

	return httpString
}

func (req *HTTPRequest) buildBytes() []byte {
	return []byte(req.build())
}

func (req *HTTPRequest) setHeader(key string, value string) *HTTPRequest {
	if req.headers == nil {
		req.headers = map[string]string{}
	}

	req.headers[key] = value

	return req
}

func (req *HTTPRequest) clear() *HTTPRequest {
	req.version = ""
	req.clearHeaders()
	req.requestType = ""
	req.path = ""
	return req
}

func (req *HTTPRequest) clearHeaders() *HTTPRequest {
	for k := range req.headers {
		delete(req.headers, k)
	}

	return req
}

func (req *HTTPRequest) setVersion(version string) *HTTPRequest {
	req.version = version
	return req
}

func (req *HTTPRequest) setReqType(reqType string) *HTTPRequest {
	req.requestType = reqType
	return req
}

func (req *HTTPRequest) checkIsValid() bool {
	return false
}

func (req *HTTPRequest) setPath(path string) *HTTPRequest {
	req.path = path
	return req
}

/*
Set data of HTTP request
*/
func (req *HTTPRequest) setData(data string) *HTTPRequest {
	req.data = data
	return req
}

const (
	OK200    = "200 OK"
	ERROR500 = "500 Internal Server Error"
	ERROR404 = "404 Not Found"
	ERROR400 = "400 Bad Request"
	ERROR401 = "401 Unauthorized"
	ERROR403 = "403 Forbidden"
	ERROR411 = "411 Length Required"
)

const (
	VERSION1_1 = "1.1"
	VERSION2   = "2"
)

const (
	REQ_GET  = "GET"
	REQ_POST = "POST"
	REQ_PUT  = "PUT"
	REQ_DEL  = "DELETE"
)

const (
	CONTENT_PLAIN = "text/plain"
	CONTENT_HTML  = "text/html"
	CONTENT_JSON  = "application/json"
)

const (
	CRLF = "\r\n"
)

const (
	HDR_CONTENT_TYPE = "Content-Type"
	HDR_CONNECTION   = "Connection"
)

type HTTPResponse struct {
	status     string
	headers    map[string]string
	data       string // json
	ctxHeaders map[string]string
}

func (res *HTTPResponse) build() string {
	dataToWriteBack := "HTTP/1.1 "
	dataToWriteBack += res.status + CRLF

	for key, val := range res.headers {
		dataToWriteBack += key + ":" + val + CRLF
	}

	contentLength := len(res.data)

	connectionType, ok := res.ctxHeaders["Connection"]

	if ok {
		if strings.ToLower(connectionType) != "keep-alive" {
			dataToWriteBack += "Connection: close" + CRLF
		} else {
			dataToWriteBack += "Connection: keep-alive" + CRLF
		}
	}

	dataToWriteBack += "Content-Length:" + strconv.Itoa(contentLength) + CRLF

	dataToWriteBack += CRLF

	dataToWriteBack += res.data

	return dataToWriteBack
}

func (res *HTTPResponse) buildBytes() []byte {
	return []byte(res.build())
}

func (res *HTTPResponse) setStatus(status string) *HTTPResponse {
	res.status = status
	return res
}

func (res *HTTPResponse) setHeader(key string, value string) *HTTPResponse {
	if res.headers == nil {
		res.headers = map[string]string{}
	}

	res.headers[key] = value
	return res
}

func (res *HTTPResponse) clearHeaders() *HTTPResponse {
	for k := range res.headers {
		delete(res.headers, k)
	}

	return res
}

func (res *HTTPResponse) setDataStr(data string) *HTTPResponse {
	res.data = data
	return res
}

func (res *HTTPResponse) setData(data any) *HTTPResponse {
	jsonBytes, _ := json.Marshal(data) // cannot fail under normal circumstances
	res.data = string(jsonBytes)

	return res
}

func (res *HTTPResponse) setCtxHeaders(ctxHeaders map[string]string) *HTTPResponse {
	res.ctxHeaders = ctxHeaders
	return res
}
