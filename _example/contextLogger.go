package main

/*
Context接口中日志相关方法。
type Context interface {
	...
	Debug(...interface{})
	Info(...interface{})
	Warning(...interface{})
	Error(...interface{})
	Fatal(...interface{})
	Debugf(string, ...interface{})
	Infof(string, ...interface{})
	Warningf(string, ...interface{})
	Errorf(string, ...interface{})
	Fatalf(string, ...interface{})
	WithField(key string, value interface{}) Logout
	WithFields(fields Fields) Logout
	Logger() Logout
}

ctx.Fatal会返回状态码500和err的内容。
例如： {"error":"fatal logger method: GET path: /err","status":"500","x-request-id":""}
*/

import (
	"github.com/eudore/eudore"
	"github.com/eudore/eudore/component/httptest"
)

func main() {
	app := eudore.NewApp()
	app.AnyFunc("/*", func(ctx eudore.Context) {
		ctx.Info("hello")
		ctx.Infof("hello path is %s", ctx.GetParam("*"))
		ctx.Warning("warning")
		ctx.Warningf("warningf")
		ctx.Error(nil)
		ctx.Errorf("test error")
		ctx.Fatal(nil)
	})
	app.AnyFunc("/err", func(ctx eudore.Context) {
		ctx.Fatalf("fatal logger method: %s path: %s", ctx.Method(), ctx.Path())
		ctx.Debug("err:", ctx.Err())
	})
	app.AnyFunc("/field", func(ctx eudore.Context) {
		ctx.Logger().WithField("key", "test-firle").Debug("debug")
		ctx.WithFields(eudore.Fields{
			"key":  "ctx.WithFields",
			"name": "eudore",
		}).Debug("hello fields")
		ctx.WithFields(nil).Debug("hello empty fields")
		ctx.WithField("key", "test-firle").Debug("debug")
		ctx.WithField("key", "test-firle").Debugf("debugf")
		ctx.WithField("key", "test-firle").Info("hello")
		ctx.WithField("key", "test-firle").Infof("hello path is %s", ctx.GetParam("*"))
		ctx.WithField("key", "test-firle").Warning("warning")
		ctx.WithField("key", "test-firle").Warningf("warningf")
		ctx.WithField("key", "test-firle").Error(nil)
		ctx.WithField("key", "test-firle").Errorf("test error")
		ctx.WithField("key", "test-firle").Fatal(nil)
		ctx.WithField("key", "test-firle").WithField("hello", "haha").Fatalf("fatal logger method: %s path: %s", ctx.Method(), ctx.Path())
		ctx.WithField("method", "WithField").WithFields(eudore.Fields{
			"key":  "ss",
			"name": "eudore",
		}).Debug("hello fields")
	})

	client := httptest.NewClient(app)
	client.NewRequest("GET", "/ffile").WithHeaderValue(eudore.HeaderAccept, eudore.MimeApplicationJSON).Do().CheckStatus(200).Out()
	client.NewRequest("GET", "/err").WithHeaderValue(eudore.HeaderAccept, eudore.MimeApplicationJSON).Do().CheckStatus(200).Out()
	client.NewRequest("GET", "/field").WithHeaderValue(eudore.HeaderAccept, eudore.MimeApplicationJSON).Do().CheckStatus(200).Out()
	for client.Next() {
		app.Error(client.Error())
	}

	app.CancelFunc()
	app.Run()
}
