package eudore

import (
	"os"
	"fmt"
	"strings"
	"net/http"
	"io/ioutil"
	"encoding/json"
	
	// etcd "github.com/coreos/etcd/client"
)


func ParseInitData(c Config) error {
	c.Set("config", "file:///data/web/golang/src/wejass/config/config-eudore.json")
	return nil
}

func ParseRead(c Config) error {
	path := c.Get("#config").(string)
	if path == "" {
		return fmt.Errorf("config data is null")
	}
	// read protocol
	// get read func
	s := strings.SplitN(path, "://", 2)
	fn := ConfigLoadConfigReadFunc(s[0])
	if fn == nil {
		// use default read func
		fmt.Println("undefined read config: " + path + ", use default.")
		fn = ConfigLoadConfigReadFunc("default")
	}
	data, err := fn(path)
	c.Set("configdata", data)
	return err
}

func ParseConfig(c Config) error {
	err := json.Unmarshal([]byte(c.Get("configdata").(string)), c.Get(""))
	//Json(string(c.Config.Data), c)
	return err	
}

func ParseArgs(c Config) (err error) {
	for _, str := range os.Args[1:] {
		if !strings.HasPrefix(str, "--") {
			// fmt.Println("invalid args", str)
			continue
		}
		c.Set(split2byte(str[2:], '='))
	}
	return
}


func ParseEnvs(c Config) error {
	for _, value := range os.Environ() {
		if strings.HasPrefix(value, "ENV_") {
			k, v := split2byte(value, '=')
			k = strings.ToLower(strings.Replace(k, "_", ".", -1))[4:]
			c.Set(k, v)
		}
	}
	return nil
}



// Read config file
func ReadFile(path string) (string, error) {
	if strings.HasPrefix(path, "file://") {
		path = path[7:]
	}
	data, err := ioutil.ReadFile(path)
	
	last := strings.LastIndex(path, ".") + 1
	if last == 0 {
		return "", fmt.Errorf("read file config, type is null")
	}
	return string(data), err
}
// Send http request get config info
func ReadHttp(path string) (string, error) {
	resp, err := http.Get(path)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	data, err := ioutil.ReadAll(resp.Body)
	return string(data), err
}
//
// example: etcd://127.0.0.1:2379/config
/*func ReadEtcd(path string) (string, error) {
	server, key := split2byte(path[7:], '/')
	cfg := etcd.Config{
		Endpoints:               []string{"http://" + server},
		Transport:               etcd.DefaultTransport,
		// set timeout per request to fail fast when the target endpoint is unavailable
		HeaderTimeoutPerRequest: time.Second,
	}
	c, err := etcd.New(cfg)
	if err != nil {
		return "", err
	}
	kapi := etcd.NewKeysAPI(c)
	resp, err := kapi.Get(context.Background(), key, nil)
	return resp.Node.Value, err
}*/