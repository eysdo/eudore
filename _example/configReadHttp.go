package main

/*
实现参考eudore.ConfigParseRead和eudore.ConfigParseConfig内容
*/

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/eudore/eudore"
)

func main() {
	app := eudore.NewApp()
	app.ParseOption(func([]eudore.ConfigParseFunc) []eudore.ConfigParseFunc {
		return []eudore.ConfigParseFunc{readHttp, eudore.ConfigParseArgs, eudore.ConfigParseEnvs, eudore.ConfigParseMods, eudore.ConfigParseWorkdir, eudore.ConfigParseHelp}
	})
	app.Set("keys.config", []string{"http://127.0.0.1:8089/xxx", "http://127.0.0.1:8088/xxx"})
	app.Set("keys.help", true)

	go func(app2 *eudore.App) {
		app := eudore.NewApp()
		app.AnyFunc("/*", func(ctx eudore.Context) {
			ctx.WriteJSON(map[string]interface{}{
				"route": "/*",
				"name":  "eudore",
			})
		})
		app.Listen(":8088")

		app2.Options(app.Parse())
		app2.CancelFunc()
		app.CancelFunc()
		app.Run()
	}(app)

	app.Run()
}

func readHttp(c eudore.Config) error {
	for _, path := range eudore.GetArrayString(c.Get("keys.config")) {
		if !strings.HasPrefix(path, "http://") && !strings.HasPrefix(path, "https://") {
			continue
		}
		resp, err := http.Get(path)
		if err == nil {
			err = json.NewDecoder(resp.Body).Decode(c)
			resp.Body.Close()
		}
		if err == nil {
			c.Set("print", "read http succes json config by "+path)
			return nil
		}
		c.Set("print", "read http fatal "+path+" error: "+err.Error())
	}
	return nil
}
