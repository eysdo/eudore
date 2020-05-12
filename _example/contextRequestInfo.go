package main

/*
Context中关于请求信息的定义。
type Context interface{
	Request() *RequestReader
	SetRequest(*RequestReader)

	Read([]byte) (int, error)
	Host() string
	Method() string
	Path() string
	RealIP() string
	RequestID() string
	Referer() string
	ContentType() string
	Istls() bool
	Body() []byte
	...
}

type RequestReader = http.Request
*/

import (
	"fmt"
	"github.com/eudore/eudore"
	"github.com/eudore/eudore/component/httptest"
)

func main() {
	app := eudore.NewApp()
	app.AnyFunc("/*", func(ctx eudore.Context) {
		ctx.WriteString("host: " + ctx.Host())
		ctx.WriteString("\nmethod: " + ctx.Method())
		ctx.WriteString("\npath: " + ctx.Path())
		ctx.WriteString("\nreal ip: " + ctx.RealIP())
		ctx.WriteString("\nreferer: " + ctx.Referer())
		ctx.WriteString("\ncontext type: " + ctx.ContentType())
		ctx.WriteString("\nistls: " + fmt.Sprint(ctx.Istls()))
		body := ctx.Body()
		if len(body) > 0 {
			ctx.WriteString("\nbody: " + string(body))
		}
	})

	client := httptest.NewClient(app)
	client.NewRequest("GET", "/").Do().Out()

	app.CancelFunc()
	app.Run()
}
