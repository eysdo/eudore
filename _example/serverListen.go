package main

import (
	"github.com/eudore/eudore"
	"net"
)

func main() {
	app := eudore.NewApp()

	// 方式1: net.Listen获得net.Listener然后启动server。
	ln, err := net.Listen("tcp", ":8086")
	if err == nil {
		app.Infof("listen %s %s", ln.Addr().Network(), ln.Addr().String())
		app.Serve(ln)
	} else {
		app.Error(err)
	}

	// 方式2: 使用eudore.ServerListenConfig配置启动监听，该方法支持eudore热重启，会进行fd的传递，需要指定启动函数startNewProcess。
	ln, err = (&eudore.ServerListenConfig{
		Addr:     ":8087",
		HTTPS:    true,
		HTTP2:    true,
		Keyfile:  "",
		Certfile: "",
	}).Listen()
	if err == nil {
		app.Infof("listen %s %s", ln.Addr().Network(), ln.Addr().String())
		app.Serve(ln)
	} else {
		app.Error(err)
	}

	(&eudore.ServerListenConfig{HTTPS: true}).Listen()
	(&eudore.ServerListenConfig{HTTPS: false}).Listen()

	// 方式3: 使用app调用eudore.ServerListenConfig启动监听。
	app.ListenTLS(":8088", "", "")
	app.CancelFunc()
	app.Run()
}
