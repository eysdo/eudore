package middleware

import (
	"bytes"
	"compress/gzip"
	"crypto/sha1"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/eudore/eudore"
)

// NewDumpFunc 函数创建一个截取请求信息的中间件，将匹配请求使用webscoket输出给客户端。
//
// router参数是eudore.Router类型，然后注入拦截路由处理。
func NewDumpFunc(router eudore.Router) eudore.HandlerFunc {
	var d dump
	router.AnyFunc("/dump/ui", HandlerAdmin)
	router.AnyFunc("/dump/connect", d.dumphandler)
	return func(ctx eudore.Context) {
		conns := d.matchConn(ctx)
		if len(conns) != 0 {
			// not handler panic
			ctx.Body()
			dumpresp := &dumpResponset{ResponseWriter: ctx.Response()}
			ctx.SetResponse(dumpresp)
			ctx.Next()
			req := ctx.Request()
			msg := &dumpMessage{
				Time:           time.Now(),
				Path:           ctx.Path(),
				Host:           ctx.Host(),
				RemoteAddr:     req.RemoteAddr,
				Proto:          req.Proto,
				Method:         req.Method,
				RequestURI:     req.RequestURI,
				RequestHeader:  req.Header,
				RequestBody:    ctx.Body(),
				Status:         ctx.Response().Status(),
				ResponseHeader: ctx.Response().Header(),
				ResponseBody:   dumpresp.GetBodyData(),
				Params:         ctx.Params(),
				Handlers:       getContextHandlerName(ctx),
			}
			msg.WriteMessage(conns)
		}
	}
}

type dump struct {
	sync.RWMutex
	dumpconn []*dumpConn
}

func (d *dump) dumphandler(ctx eudore.Context) {
	err := d.newDumpConn(ctx)
	if err != nil {
		ctx.Fatal(err)
	}
	ctx.End()
}

func (d *dump) newDumpConn(ctx eudore.Context) error {
	d.Lock()
	defer d.Unlock()
	conn, buf, err := ctx.Response().Hijack()
	if err != nil {
		return err
	}
	h := sha1.New()
	h.Write([]byte(ctx.GetHeader("Sec-WebSocket-Key") + "258EAFA5-E914-47DA-95CA-C5AB0DC85B11"))
	buf.Write([]byte("HTTP/1.1 101\r\nConnection: Upgrade\r\nUpgrade: websocket\r\nSec-WebSocket-Accept: "))
	buf.Write([]byte(base64.StdEncoding.EncodeToString(h.Sum(nil))))
	buf.Write([]byte("\r\nX-Eudore-Admin: dump\r\n\r\n"))
	buf.Flush()

	querys := ctx.Querys()
	dumpconn := &dumpConn{
		Conn: conn,
		keys: make([]string, 0, len(querys)),
		vals: make([]string, 0, len(querys)),
	}
	for k := range querys {
		dumpconn.keys = append(dumpconn.keys, k)
		dumpconn.vals = append(dumpconn.vals, querys.Get(k))
	}
	for i := 0; i < len(d.dumpconn); i++ {
		d.dumpconn[i].Lock()
		empty := d.dumpconn[i].Conn == nil
		d.dumpconn[i].Unlock()
		if empty {
			d.dumpconn[i] = dumpconn
			return nil
		}
	}
	d.dumpconn = append(d.dumpconn, dumpconn)
	return nil
}

func (d *dump) matchConn(ctx eudore.Context) (conns []*dumpConn) {
	d.RLock()
	defer d.RUnlock()
	for i := 0; i < len(d.dumpconn); i++ {
		if d.dumpconn[i].Match(ctx) {
			conns = append(conns, d.dumpconn[i])
		}
	}
	return
}

type dumpConn struct {
	sync.Mutex
	net.Conn
	keys []string
	vals []string
}

func (cond *dumpConn) Match(ctx eudore.Context) bool {
	cond.Lock()
	defer cond.Unlock()
	if cond.Conn == nil {
		return false
	}
	for i := range cond.keys {
		if cond.keys[i] == "path" && !matchStar(ctx.Path(), cond.vals[i]) ||
			strings.HasPrefix(cond.keys[i], "query-") && !matchStar(ctx.GetParam(cond.keys[i][6:]), cond.vals[i]) ||
			strings.HasPrefix(cond.keys[i], "header-") && !matchStar(ctx.GetHeader(cond.keys[i][7:]), cond.vals[i]) ||
			strings.HasPrefix(cond.keys[i], "param-") && !matchStar(ctx.GetParam(cond.keys[i][6:]), cond.vals[i]) {
			return false
		}
	}
	return true
}

type dumpMessage struct {
	Time           time.Time
	Path           string
	Host           string
	RemoteAddr     string
	Proto          string
	Method         string
	RequestURI     string
	RequestHeader  http.Header
	RequestBody    []byte
	Status         int
	ResponseHeader http.Header
	ResponseBody   []byte
	Params         *eudore.Params
	Handlers       []string
}

func (msg *dumpMessage) WriteMessage(conns []*dumpConn) {
	body, _ := json.Marshal(msg)
	var head []byte
	length := len(body)
	if length <= 0xffff {
		head = []byte{129, 126, uint8(length >> 8), uint8(length & 0xff)}
	} else {
		head = []byte{129, 127, 0, 0, 0, 0, 0, 0, 0, 0}
		for i := uint(0); i < 7; i++ {
			head[9-i] = uint8(length >> (8 * i) & 0xff)
		}
	}
	for _, conn := range conns {
		conn.Lock()
		conn.Write(head)
		_, err := conn.Write(body)
		if err != nil {
			conn.Conn = nil
		}
		conn.Unlock()
	}
}

func getContextHandlerName(ctx eudore.Context) []string {
	_, handlers := ctx.GetHandler()
	names := make([]string, len(handlers))
	for i := range handlers {
		names[i] = fmt.Sprint(handlers[i])
	}
	return names
}

type dumpResponset struct {
	eudore.ResponseWriter
	bytes.Buffer
}

// Write 方法实现ResponseWriter中的Write方法。
func (w *dumpResponset) Write(data []byte) (int, error) {
	w.Buffer.Write(data)
	return w.ResponseWriter.Write(data)
}

// GetBodyData 方法获取写入的body内容，如果是gzip编码则解压。
func (w *dumpResponset) GetBodyData() []byte {
	if w.ResponseWriter.Header().Get(eudore.HeaderContentEncoding) == "gzip" {
		gread := new(gzip.Reader)
		gread.Reset(&w.Buffer)
		body, err := ioutil.ReadAll(gread)
		if err != nil {
			return w.Buffer.Bytes()
		}
		gread.Close()
		return body
	}
	return w.Buffer.Bytes()
}
