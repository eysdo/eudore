package httptest

import (
	"bufio"
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"mime/multipart"
	"net"
	"net/http"
	"net/url"
	"os"
	"strings"
)

// RequestReaderTest 实现protocol.RequestReader接口，用于执行测试请求。
type RequestReaderTest struct {
	//
	Client *Client
	File   string
	Line   int
	err    error
	// data
	*http.Request
	websocketHandle func(net.Conn)
	websocketClient net.Conn
	websocketServer net.Conn
	json            interface{}
	formValue       map[string][]string
	formFile        map[string][]fileContent
}
type fileContent struct {
	Name string
	io.Reader
}

// NewRequestReaderTest 函数创建一个测试http请求。
func NewRequestReaderTest(client *Client, method, path string) *RequestReaderTest {
	r := &RequestReaderTest{
		Client: client,
	}
	r.File, r.Line = logFormatFileLine(3)
	u, err := url.ParseRequestURI(path)
	if err != nil {
		u = new(url.URL)
	}
	if u.Host == "" {
		u.Host = HTTPTestHost
	}
	r.Request = &http.Request{
		Method:     method,
		RequestURI: u.RequestURI(),
		URL:        u,
		Proto:      "HTTP/1.1",
		ProtoMajor: 1,
		ProtoMinor: 1,
		Header:     make(http.Header),
		Host:       HTTPTestHost,
	}
	if method == "ws" || method == "wss" {
		if u.Scheme == "" {
			u.Scheme = method
		}
		r.Method = "GET"
		r.Header.Set("Host", r.Host)
		r.Header.Add("Upgrade", "websocket")
		r.Header.Add("Connection", "Upgrade")
		r.Header.Add("Sec-WebSocket-Key", "x3JJHMbDL1EzLkh9GBhXDw==")
		r.Header.Add("Sec-WebSocket-Version", "13")
		r.Header.Add("Origin", "http://"+r.Host)
		r.Body = http.NoBody
	}
	r.Form, _ = url.ParseQuery(r.URL.RawQuery)
	if err != nil {
		r.Error(err)
	}
	return r
}

func (r *RequestReaderTest) Error(err error) {
	if r.err == nil {
		r.err = err
	}
	r.Errorf("%s", err.Error())
}

// Errorf 方法输出错误信息。
func (r *RequestReaderTest) Errorf(format string, args ...interface{}) {
	err := fmt.Errorf(format, args...)
	err = fmt.Errorf("httptest request %s %s of file location %s:%d, error: %v", r.Method, r.RequestURI, r.File, r.Line, err)
	r.Client.Errs = append(r.Client.Errs, err)
}

// WithAddQuery 方法给请求添加一个url参数。
func (r *RequestReaderTest) WithAddQuery(key, val string) *RequestReaderTest {
	r.Form.Add(key, val)
	return r
}

// WithHeaders 方法给请求添加多个header。
func (r *RequestReaderTest) WithHeaders(headers http.Header) *RequestReaderTest {
	for key, vals := range headers {
		for _, val := range vals {
			r.Request.Header.Add(key, val)
		}
	}
	return r
}

// WithHeaderValue 方法给请求添加一个header的值。
func (r *RequestReaderTest) WithHeaderValue(key, val string) *RequestReaderTest {
	r.Request.Header.Add(key, val)
	return r
}

// WithBody 方法设置请求的body。
func (r *RequestReaderTest) WithBody(reader interface{}) *RequestReaderTest {
	body, err := transbody(reader)
	if err != nil {
		r.Errorf("%v", err)
	} else if body != nil {
		r.Request.Body = ioutil.NopCloser(body)
	}
	return r
}

func transbody(body interface{}) (io.Reader, error) {
	if body == nil {
		return nil, nil
	}
	switch t := body.(type) {
	case string:
		return strings.NewReader(t), nil
	case []byte:
		return bytes.NewReader(t), nil
	case io.Reader:
		return t, nil
	default:
		return nil, fmt.Errorf("unknown type used for body: %+v", body)
	}
}

// WithBodyString 方法设置请求的字符串body。
func (r *RequestReaderTest) WithBodyString(s string) *RequestReaderTest {
	r.Body = ioutil.NopCloser(strings.NewReader(s))
	r.ContentLength = int64(len(s))
	return r
}

// WithBodyByte 方法设置请的字节body。
func (r *RequestReaderTest) WithBodyByte(b []byte) *RequestReaderTest {
	r.Body = ioutil.NopCloser(bytes.NewReader(b))
	r.ContentLength = int64(len(b))
	return r
}

// WithBodyJSON 方法设置body为一个对象的json字符串。
func (r *RequestReaderTest) WithBodyJSON(data interface{}) *RequestReaderTest {
	r.json = data
	return r
}

// WithBodyJSONValue 方法设置一条json数据，使用map[string]interface{}保存json数据。
func (r *RequestReaderTest) WithBodyJSONValue(key string, val interface{}, args ...interface{}) *RequestReaderTest {
	if r.json == nil {
		r.json = make(map[string]interface{})
	}
	data, ok := r.json.(map[string]interface{})
	if !ok {
		return r
	}
	data[key] = val
	args = initSlice(args)
	for i := 0; i < len(args); i += 2 {
		data[fmt.Sprint(args[i])] = args[i+1]
	}
	return r
}

// WithBodyFormValue 方法使用Form表单，添加一条键值数据。
func (r *RequestReaderTest) WithBodyFormValue(key, val string, args ...string) *RequestReaderTest {
	if r.formValue == nil {
		r.formValue = make(map[string][]string)
	}
	r.formValue[key] = append(r.formValue[key], val)

	args = initSliceSrting(args)
	for i := 0; i < len(args); i += 2 {
		r.formValue[args[i]] = append(r.formValue[args[i]], args[i+1])
	}
	return r
}

// WithBodyFormValues 方法使用Form表单，添加多条键值数据。
func (r *RequestReaderTest) WithBodyFormValues(data map[string][]string) *RequestReaderTest {
	if r.formValue == nil {
		r.formValue = make(map[string][]string)
	}
	for key, vals := range data {
		r.formValue[key] = vals
	}
	return r
}

// WithBodyFormFile 方法使用Form表单，添加一个文件名称和内容。
func (r *RequestReaderTest) WithBodyFormFile(key, name string, val interface{}) *RequestReaderTest {
	if r.formFile == nil {
		r.formFile = make(map[string][]fileContent)
	}

	body, err := transbody(val)
	if err != nil {
		r.Error(err)
		return r
	}

	r.formFile[key] = append(r.formFile[key], fileContent{name, body})
	return r
}

// WithBodyFormLocalFile 方法设置请求body Form的文件，值为实际文件路径
func (r *RequestReaderTest) WithBodyFormLocalFile(key, name, path string) *RequestReaderTest {
	if r.formFile == nil {
		r.formFile = make(map[string][]fileContent)
	}

	file, err := os.Open(path)
	if err != nil {
		r.Error(err)
		return r
	}

	r.formFile[key] = append(r.formFile[key], fileContent{name, file})
	return r
}

// WithWebsocket 方法定义websock处理函数。
func (r *RequestReaderTest) WithWebsocket(fn func(net.Conn)) *RequestReaderTest {
	r.websocketHandle = fn
	return r
}

// Do 方法发送这个请求，使用客户端处理这个请求返回响应。
func (r *RequestReaderTest) Do() *ResponseWriterTest {
	if r.err != nil {
		resp := NewResponseWriterTest(r.Client, r)
		resp.Code = 500
		resp.Body = bytes.NewBufferString(r.err.Error())
		return resp
	}
	r.initArgs()
	r.initBody()
	ctx, cancel := context.WithCancel(r.Request.Context())
	defer cancel()
	r.Request = r.Request.WithContext(ctx)

	// 创建响应并处理
	resp := NewResponseWriterTest(r.Client, r)
	if r.URL.Host == HTTPTestHost {
		if r.URL.Scheme == "ws" || r.URL.Scheme == "wss" {
			r.websocketServer, r.websocketClient = net.Pipe()
		}
		r.RemoteAddr = "192.0.2.1:1234"
		r.Client.Handler.ServeHTTP(resp, r.Request)
		r.Client.CookieJar.SetCookies(r.URL, (&http.Response{Header: resp.Header()}).Cookies())
	} else {
		r.RequestURI = ""
		httpResp, err := r.sendResponse()
		if err == nil {
			resp.HandleRespone(httpResp)
		} else {
			r.Error(err)
			resp.Code = 500
			resp.Body = bytes.NewBufferString(r.err.Error())
		}

	}
	return resp
}

func (r *RequestReaderTest) initArgs() {
	// 附加客户端公共参数
	for key, vals := range r.Client.Args {
		for _, val := range vals {
			r.Request.Form.Add(key, val)
		}
	}
	r.Request.URL.RawQuery = r.Form.Encode()
	r.Form = nil

	for key, vals := range r.Client.Headers {
		for _, val := range vals {
			r.Request.Header.Add(key, val)
		}
	}
	// set host
	host := r.Header.Get("Host")
	if host != "" {
		r.Request.Host = host
		r.Header.Del("Host")
	}
	// set cookie header
	for _, cookie := range r.Client.CookieJar.Cookies(r.URL) {
		r.Request.Header.Add("Cookie", cookie.String())
	}
}

func (r *RequestReaderTest) initBody() {
	switch {
	case r.json != nil:
		r.Request.Header.Add("Content-Type", "application/json")
		reader, writer := io.Pipe()
		r.Request.Body = reader
		go func() {
			json.NewEncoder(writer).Encode(r.json)
			writer.Close()
		}()
	case r.formValue != nil || r.formFile != nil:
		reader, writer := io.Pipe()
		r.Request.Body = reader
		w := multipart.NewWriter(writer)
		r.Request.Header.Add("Content-Type", w.FormDataContentType())
		go func() {
			for key, vals := range r.formValue {
				for _, val := range vals {
					w.WriteField(key, val)
				}
			}
			for key, vals := range r.formFile {
				for _, val := range vals {
					part, _ := w.CreateFormFile(key, val.Name)
					io.Copy(part, val)
					cr, ok := val.Reader.(io.Closer)
					if ok {
						cr.Close()
					}
				}
			}
			w.Close()
			writer.Close()
		}()
	case r.Request.Body == nil:
		r.Request.Body = http.NoBody
		r.Request.ContentLength = -1
	}
}

func (r *RequestReaderTest) sendResponse() (*http.Response, error) {
	if r.URL.Scheme != "ws" && r.URL.Scheme != "wss" {
		return r.Client.Do(r.Request)
	}

	conn, err := r.dialConn()
	if err != nil {
		return nil, err
	}
	err = r.Request.Write(conn)
	if err != nil {
		return nil, err
	}
	resp, err := http.ReadResponse(bufio.NewReader(conn), r.Request)
	if err == nil {
		r.websocketHandle(conn)
	}
	return resp, err
}

var zeroDialer net.Dialer

func (r *RequestReaderTest) dialConn() (net.Conn, error) {
	ts := new(http.Transport)
	if r.Client.Transport != nil {
		ts = r.Client.Transport.(*http.Transport)
	}

	if r.URL.Scheme == "ws" {
		if ts.DialContext != nil {
			return ts.DialContext(r.Request.Context(), "tcp", r.Request.URL.Host)
		}
		if ts.Dial != nil {
			return ts.Dial("tcp", r.Request.URL.Host)
		}
		return zeroDialer.DialContext(r.Request.Context(), "tcp", r.Request.URL.Host)
	}
	// by go1.14
	// if ts.DialTLSContext  != nil {
	// 	return ts.DialTLSContext(r.Request.Context(), "tcp", r.Request.Host)
	// }
	if ts.DialTLS != nil {
		return ts.DialTLS("tcp", r.Request.URL.Host)
	}
	return tls.Dial("tcp", r.Request.URL.Host, &tls.Config{InsecureSkipVerify: true})
}

func initSlice(args []interface{}) []interface{} {
	if len(args)%2 == 0 {
		return args
	}
	return args[:len(args)-1]
}

func initSliceSrting(args []string) []string {
	if len(args)%2 == 0 {
		return args
	}
	return args[:len(args)-1]
}
