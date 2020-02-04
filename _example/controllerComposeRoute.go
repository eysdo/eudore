package main

/*
如果控制器嵌入一个名称为Controller为前缀的属性，该对象的全部方法不会自动注册路由，否在可以嵌入获得该对象的方法注册成路由。
*/

import (
	"github.com/eudore/eudore"
	"github.com/eudore/eudore/component/httptest"
)

type (
	tableController struct {
		eudore.ControllerBase
	}
	// myRouteController 从tableController嵌入两个方法注册成路由。
	myRouteController struct {
		tableController
	}
)

func main() {
	app := eudore.NewCore()
	app.AddController(new(myRouteController))

	client := httptest.NewClient(app)
	client.NewRequest("GET", "/mybase/hello").Do().Out()
	client.NewRequest("PUT", "/mybase/").Do().Out()
	for client.Next() {
		app.Error(client.Error())
	}
	client.Stop(0)

	app.Listen(":8088")
	app.Run()
}

func (ctl *tableController) Hello() interface{} {
	return "hello eudore"
}

func (ctl *tableController) Any() {
	ctl.Debug("tableController Any", ctl.Hello())
}
