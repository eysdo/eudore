package main

/*
eudore.App默认Logger为LoggerInit对象，会保存全部日志信息，在调用NextHandler方法时，将全部日志输出给新日志。

LoggerInit意义是将配置解析之前，未设置Logger的日子全部保存起来，来初始化Logger后处理之前的日志，在调用SetLevel方法后，在NextHandler方法会传递日志级别。

如果修改Logger、Server、Router后需要调用Set方法重写，设置目标的输出函数。
*/

import (
	"github.com/eudore/eudore"
)

func main() {
	app := eudore.NewApp(eudore.NewLoggerInit())
	app.Debug(0)
	app.Info(1)
	app.Sync()
	app.Info(2)
	app.Info(3)
	app.AnyFunc("/*path", eudore.HandlerEmpty)
	app.Options(eudore.NewLoggerStd(nil))
	app.CancelFunc()
	app.Run()
}
