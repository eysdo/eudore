package notify

import (
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/eudore/eudore"
	"github.com/fsnotify/fsnotify"
)

var notifyArgs = []string{
	fmt.Sprintf("%s=%d", eudore.EnvEudoreIsNotify, 1),
	fmt.Sprintf("%s=%d", eudore.EnvEudoreDisablePidfile, 1),
}
var startcmd string

func init() {
	if runtime.GOOS == "windows" {
		startcmd = "powershell"
	} else {
		startcmd = "bash"
	}
}

// Notify 定义监听重启对象。
type Notify struct {
	sync.Mutex
	app         *eudore.App
	watcher     *fsnotify.Watcher
	buildCmd    []string
	startCmd    []string
	watchDir    []string
	lastBuild   context.CancelFunc
	lastProcess context.CancelFunc
}

// NewNotify 函数创建一个Notify对象。
func NewNotify(app *eudore.App) *Notify {
	if app.Config.Get("component.notify.disable") != nil {
		app.Info("notify is disable")
		return nil
	}
	var (
		buildCmd = getArgs(app.Config.Get("component.notify.buildcmd"))
		startCmd = getArgs(app.Config.Get("component.notify.startcmd"))
		watchDir = getArgs(app.Config.Get("component.notify.watchdir"))
	)

	if len(buildCmd) == 0 {
		app.Info("notify build command is empty.")
		return nil
	}

	if len(startCmd) == 0 {
		startCmd = os.Args
	}

	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		panic(err)
	}

	return &Notify{
		app:      app,
		watcher:  watcher,
		buildCmd: buildCmd,
		startCmd: startCmd,
		watchDir: watchDir,
	}
}

// IsRun 方法返回Notify是否可以启动。
func (n *Notify) IsRun() bool {
	return !(eudore.GetStringBool(os.Getenv(eudore.EnvEudoreIsNotify)) || n == nil)
}

// Run 方法启动Notify。
//
// 调用App.Logger
func (n *Notify) Run() error {
	if eudore.GetStringBool(os.Getenv(eudore.EnvEudoreIsNotify)) || n == nil {
		return nil
	}
	n.app.Info("notify buildCmd", n.buildCmd)
	n.app.Info("notify startCmd", n.startCmd)
	for _, i := range n.watchDir {
		n.WatchAll(i)
	}

	n.buildAndRestart()

	var timer = time.AfterFunc(1000*time.Hour, n.buildAndRestart)
	defer func() {
		timer.Stop()
		if n.lastBuild != nil {
			n.lastBuild()
		}
		if n.lastProcess != nil {
			n.lastProcess()
		}
	}()

	for {
		select {
		case event, ok := <-n.watcher.Events:
			if !ok {
				break
			}

			// 监听go文件写入
			if event.Name[len(event.Name)-3:] == ".go" && event.Op&fsnotify.Write == fsnotify.Write {
				n.app.Debug("modified file:", event.Name)

				// 等待0.1秒执行更新，防止短时间大量触发
				timer.Reset(100 * time.Millisecond)
			}
		case err, ok := <-n.watcher.Errors:
			if !ok {
				break
			}
			n.app.Error("notify watcher error:", err)
		case <-n.app.Done():
			return eudore.ErrApplicationStop
		}
	}
}

func (n *Notify) buildAndRestart() {
	// 取消上传编译
	n.Lock()
	if n.lastBuild != nil {
		n.lastBuild()
	}
	ctx, cannel := context.WithCancel(n.app.Context)
	n.lastBuild = cannel
	n.Unlock()
	// 执行编译命令
	cmd := exec.CommandContext(ctx, startcmd, "-c", strings.Join(n.buildCmd, " "))
	cmd.Env = os.Environ()
	body, err := cmd.CombinedOutput()
	if err != nil {
		fmt.Printf("notify build error: \n%s", body)
		n.app.Errorf("notify build error: %s", body)
	} else {
		n.app.Info("notify build success, restart process...")
		time.Sleep(10 * time.Millisecond)
		// 重启子进程
		n.restart()
	}
}

func (n *Notify) restart() {
	// 关闭旧进程
	n.Lock()
	if n.lastProcess != nil {
		n.lastProcess()
	}
	ctx, cannel := context.WithCancel(n.app.Context)
	n.lastProcess = cannel
	n.Unlock()
	// 启动新进程
	cmd := exec.CommandContext(ctx, n.startCmd[0], n.startCmd[1:]...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Env = append(os.Environ(), notifyArgs...)
	err := cmd.Start()
	if err != nil {
		n.app.Error("notify start error:", err)
	}
}

// WatchAll 方法添加一个文件或目录，如果/结尾的目录会递归监听子目录。
func (n *Notify) WatchAll(path string) {
	// 递归目录处理
	if path[len(path)-1] == '/' {
		listDir(path, n.watch)
	}
	n.watch(path)
}

func (n *Notify) watch(path string) {
	n.app.Debug("notify add watch dir " + path)
	err := n.watcher.Add(path)
	if err != nil {
		n.app.Error(err)
	}
}

func listDir(path string, fn func(string)) {
	files, _ := ioutil.ReadDir(path)
	for _, f := range files {
		// 忽略隐藏目录，例如: .git
		if f.IsDir() && f.Name()[0] != '.' {
			path := filepath.Join(path, f.Name())
			fn(path)
			listDir(path, fn)
		}
	}
}

// 转换配置成数组类型
func getArgs(i interface{}) []string {
	if strs, ok := i.([]string); ok {
		return strs
	}
	if strs, ok := i.(string); ok {
		return strings.Split(strs, " ")
	}
	return nil
}
