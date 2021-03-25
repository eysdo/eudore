package eudore

import (
	"fmt"
	"reflect"
	"strings"
	"sync"
)

// Controller 定义控制器必要的接口。
//
// 控制器默认具有Base、Data、Singleton、View四种实现。
//
// Base是最基本的控制器实现；
// Data在Base基础上使用ContextData作为请求上下文，默认具有更多的方法；
// Singleton是单例控制器，每次请求公用一个控制器；
// View在Base基础上会使用Data和推断的模板渲染view。
//
// 控制器组合一个名称为xxxController，会组合获得xxx控制器的路由方法。
type Controller interface {
	Init(Context) error
	Release(Context) error
	Inject(Controller, Router) error
}

// controllerRoute 定义获得路由和方法映射的接口。
type controllerRoute interface {
	ControllerRoute() map[string]string
}

// controllerRouteParam 定义获得一个路由参数的接口，转入pkg、controllername、methodname获得需要添加的路由参数。
type controllerRouteParam interface {
	GetRouteParam(string, string, string) string
}

// ControllerFuncExtend 定义控制器函数扩展使用的信息，需要在处理函数扩展中注册对应的处理函数
//
// ControllerInjectStateful和ControllerInjectSingleton 函数都会使用ControllerFuncExtend对象注册扩展函数。
type ControllerFuncExtend struct {
	Name       string
	Index      int
	Controller Controller
	Pool       ControllerPool
}

// ControllerPool 定义控制器池，使ControllerFuncExtend兼容有状态和单例使用的控制器对象。
type ControllerPool interface {
	Get() Controller
	Put(Controller)
}

// controllerPoolSync 定义sync控制器池，用于有状态控制器使用。
type controllerPoolSync struct {
	sync.Pool
}

// controllerPoolSingleton 定义单例控制器池，用于单例控制器返回单例对象。
type controllerPoolSingleton struct {
	Controller
}

// virtualController 是一个空控制器用于组合实现空方法。
type virtualController struct{}

// ControllerBase 实现基本控制器。
type ControllerBase struct {
	Context
	virtualController
}

// ControllerData 实现基于ContextData的控制器,基于ControllerBase扩展了额外的控制器方法。
type ControllerData struct {
	ContextData
	virtualController
}

// ControllerSingleton 实现单例控制器。
type ControllerSingleton struct{ virtualController }

// ControllerAutoRoute 实现根据方法注册对应的路由器方法。
type ControllerAutoRoute struct{ virtualController }

// ControllerView 基于ControllerBase额外增加了控制器自动渲染数据。
//
// 默认模板路由可以通过重写GetRouteParam方法，重新定义template参数。
//
// 如果Data不为空且未写入数据，会调用Render渲染数据。
//
// 如果渲染出html需要app.Renderer支持。
type ControllerView struct {
	ContextData
	Data map[string]interface{}
}

type controllerError struct {
	Error error
	Name  string
	virtualController
}

// NewControllerError 函数返回一个控制器错误，在控制器Inject时返回对应的错误。
func NewControllerError(ctl Controller, err error) Controller {
	return &controllerError{
		Error: err,
		Name:  getConrtrollerName(ctl),
	}
}

// Inject 方法在注入路由规则时返回控制器错误。
func (ctl *controllerError) Inject(Controller, Router) error {
	return ctl.Error
}

// String 方法返回controllerError的控制器名称。
func (ctl *controllerError) String() string {
	return ctl.Name
}

// ControllerInjectStateful 函数执行的每次控制器会使用sync.Pool分配和回收。
func ControllerInjectStateful(controller Controller, router Router) error {
	return ControllerInjectWithPool(NewControllerPoolStateful(controller), controller, router)
}

// ControllerInjectSingleton 函数每次控制器会使用同一个对象执行请求，注意控制器数据线程安全。
func ControllerInjectSingleton(controller Controller, router Router) error {
	return ControllerInjectWithPool(NewControllerPoolSingleton(controller), controller, router)
}

// ControllerInjectWithPool 定义基本的控制器实现函数。
//
// 如果控制器名称为XxxxController，控制器会注册到路由组/Xxxx下，注册的方法会附加请求上下文参数'controller'，指定控制器包名称。
//
// 请求方法为函数首字母大写单词，如果方法不是Get、Post、Put、Delete、Patch、Options方法，则使用Any方法。
//
// 请求路径为名称每首字母大写单词组成，忽略第一个表示请求方法的单词，如果前一个单位为'By'表示是变量。
//
// Hello() ANY /hello
//
// Get() GET /*
//
// GetId() GET /id
//
// GetById() GET /:id
//
// 如果控制器实现ControllerRoute接口，会替换自动分析路由路径，路由路径为空会忽略该方法。
//
// 如果控制器实现interface{GetRouteParam(string, string, string) string}接口，使用改接口方法来生成路由参数。
//
// 如果控制器嵌入了其他基础控制器(控制器名称为:ControllerXxx)，控制器路由分析会忽略嵌入的控制器的全部方法。
//
// 如果控制器具有非空和导出的Chan、Func、Interface、Map、Ptr、Slice、Array类型的成员，会知道赋值给新控制器。
//
// 方法类型可以调用ListExtendControllerHandlerFunc()函数查看
func ControllerInjectWithPool(pool ControllerPool, controller Controller, router Router) error {
	pType := reflect.TypeOf(controller)
	iType := reflect.TypeOf(controller).Elem()

	// 添加控制器组。
	cname := iType.Name()
	cpkg := iType.PkgPath()
	router = router.Group(getContrllerRouterGroup(cname, router))

	// 获取路由参数函数
	pfn := defaultRouteParam
	v, ok := controller.(controllerRouteParam)
	if ok {
		pfn = v.GetRouteParam
	}

	// 路由器注册控制器方法
	for method, path := range getRoutesWithName(controller) {
		m, ok := pType.MethodByName(method)
		if !ok || (!checkAllowMethod(method) && path == "") {
			continue
		}

		router.AddHandler(getRouteMethod(method), path+" "+pfn(cpkg, cname, method), ControllerFuncExtend{
			Controller: controller,
			Name:       fmt.Sprintf("%s.%s.%s", cpkg, cname, method),
			Index:      m.Index,
			Pool:       pool,
		})
	}
	return nil
}

// ControllerInjectAutoRoute 函数基于控制器规则生成路由规则，使用方法转换成处理函数支持路由器。
//
// 与ControllerInjectSingleton差别在于，AutoRoute使用处理函数扩展，Singletons使用控制器扩展。
func ControllerInjectAutoRoute(controller Controller, router Router) error {
	pType := reflect.TypeOf(controller)
	pValue := reflect.ValueOf(controller)
	iType := reflect.TypeOf(controller).Elem()

	// 添加控制器组。
	cname := iType.Name()
	cpkg := iType.PkgPath()
	router = router.Group(getContrllerRouterGroup(cname, router))

	// 获取路由参数函数
	pfn := defaultRouteParam
	v, ok := controller.(controllerRouteParam)
	if ok {
		pfn = v.GetRouteParam
	}

	// 路由器注册控制器方法
	for method, path := range getRoutesWithName(controller) {
		m, ok := pType.MethodByName(method)
		if !ok || (!checkAllowMethod(method) && path == "") {
			continue
		}

		h := pValue.Method(m.Index)
		SetHandlerAliasName(h, fmt.Sprintf("%s.%s.%s", cpkg, cname, method))
		router.AddHandler(getRouteMethod(method), path+" "+pfn(cpkg, cname, method), h)
	}
	return nil
}

func getContrllerRouterGroup(name string, router Router) string {
	switch {
	case router.Params().Get(ParamControllerGroup) != "":
		group := router.Params().Get(ParamControllerGroup)
		router.Params().Del(ParamControllerGroup)
		if group[0] == '/' {
			return group
		}
		return "/" + group
	case controllerHasSuffix(name):
		return "/" + strings.ToLower(name[:len(name)-10])
	default:
		return ""
	}
}

// defaultRouteParam 函数定义默认的控制器参数，可以通过实现controllerRouteParam来覆盖该函数。
func defaultRouteParam(pkg, name, method string) string {
	return fmt.Sprintf("controllername=%s.%s controllermethod=%s", pkg, name, method)
}

// getRoutesWithName 函数获得一个控制器类型注入的全部名称和路由路径的映射。
func getRoutesWithName(controller Controller) map[string]string {
	iType := reflect.TypeOf(controller)
	names := getContrllerAllMethos(iType)
	routes := make(map[string]string, len(names))
	for _, name := range names {
		if name != "" {
			routes[name] = getRouteByName(name)
		}
	}
	for _, name := range getContrllerIgnoreMethos(iType) {
		delete(routes,name)
	}

	// 如果控制器实现ControllerRoute接口，加载额外路由。
	controllerRoute, isRoute := controller.(controllerRoute)
	if isRoute {
		for name, path := range controllerRoute.ControllerRoute() {
			if len(path) > 0 && path[0] == ' ' {
				// ControllerRoute获得的路径是空格开头，表示为路由参数。
				routes[name] += path
			} else {
				routes[name] = path
			}
		}
	}
	return routes
}

func getContrllerIgnoreMethos(iType reflect.Type) []string {
	var allname []string
	if iType.Kind() == reflect.Ptr {
		iType = iType.Elem()
	}
	if iType.Kind() == reflect.Struct {
		for i := 0; i < iType.NumField(); i++ {
			// Controller为前缀的嵌入控制器。
			// 判断嵌入属性
			if iType.Field(i).Anonymous  {
				var ignore []string
				if controllerHasSuffix(getReflectTypeName(iType.Field(i).Type)) {
					ignore = getContrllerIgnoreMethos(iType.Field(i).Type)
				}else {
					ignore = getContrllerAllMethos(iType.Field(i).Type)
				}
				allname= append(allname, ignore...)
			}
		}
	}
	return allname
}

// controllerHasSuffix 函数判断控制器名称后缀是否为"Controller"或"controller"。
func controllerHasSuffix(name string) bool {
	return strings.HasSuffix(name, "Controller") || strings.HasSuffix(name, "controller")
}

// getContrllerAllMethos 函数获得一共类型包含指针类型的全部方法名称。
func getContrllerAllMethos(iType reflect.Type) []string {
	if iType.Kind() != reflect.Ptr {
		iType = reflect.New(iType).Type()
	}
	names := make([]string, iType.NumMethod())
	for i := 0; i < iType.NumMethod(); i++ {
		names[i] = iType.Method(i).Name
	}
	return names
}

func getReflectTypeName(iType reflect.Type) string {
	if iType.Kind() == reflect.Ptr {
		iType = iType.Elem()
	}
	return iType.Name()
}

// getRouteByName 函数使用函数名称生成路由路径。
func getRouteByName(name string) string {
	names := splitName(name)
	if checkAllowMethod(names[0]) {
		names = names[1:]
	}
	if len(names) == 0 {
		return "/*"
	}
	name = ""
	for i := 0; i < len(names); i++ {
		if names[i] == "By" {
			i++
			if i == len(names) {
				name = name + "/*"
			} else {
				name = name + "/:" + names[i]
			}
		} else {
			name = name + "/" + names[i]
		}
	}
	return strings.ToLower(name)
}

func getRouteMethod(name string) string {
	method := getFirstUp(name)
	if checkAllowMethod(method) {
		return strings.ToUpper(method)
	}
	return "ANY"
}

func getFirstUp(name string) string {
	for i, c := range name {
		if 0x40 < c && c < 0x5B && i != 0 {
			return name[:i]
		}
	}
	return name
}

func checkAllowMethod(method string) bool {
	for _, i := range []string{"Any", "Get", "Post", "Put", "Delete", "Head", "Patch", "Options", "Connect", "Trace"} {
		if i == method {
			return true
		}
	}
	return false
}

// splitName 方法基于路径首字符大写切割
func splitName(name string) (strs []string) {
	var head int
	for i, c := range name {
		if 0x40 < c && c < 0x5B && i != 0 {
			strs = append(strs, name[head:i])
			head = i
		}
	}
	strs = append(strs, name[head:])
	return
}

// Init 实现控制器初始方法。
func (ctl *virtualController) Init(ctx Context) error {
	return nil
}

// Release 实现控制器释放方法。
func (ctl *virtualController) Release(Context) error {
	return nil
}

// ControllerRoute 方法返回默认路由信息。
func (ctl *virtualController) ControllerRoute() map[string]string {
	return nil
}

// GetRouteParam 方法添加路由参数信息。
func (ctl *virtualController) GetRouteParam(pkg, name, method string) string {
	return defaultRouteParam(pkg, name, method)
}

// Init 实现控制器初始方法。
func (ctl *ControllerBase) Init(ctx Context) error {
	ctl.Context = ctx
	return nil
}

// Inject 方法实现控制器注入到路由器的方法，ControllerBase控制器调用ControllerInjectStateful方法注入。
func (ctl *ControllerBase) Inject(controller Controller, router Router) error {
	return ControllerInjectStateful(controller, router)
}

// Init 实现控制器初始方法。
func (ctl *ControllerData) Init(ctx Context) error {
	ctl.ContextData.Context = ctx
	return nil
}

// Inject 方法实现控制器注入到路由器的方法，ControllerData控制器调用ControllerInjectStateful方法注入。
func (ctl *ControllerData) Inject(controller Controller, router Router) error {
	return ControllerInjectStateful(controller, router)
}

// Inject 方法实现控制器注入到路由器的方法，ControllerSingleton控制器调用ControllerInjectSingleton方法注入。
func (ctl *ControllerSingleton) Inject(controller Controller, router Router) error {
	return ControllerInjectSingleton(controller, router)
}

// Inject 方法实现控制器注入到路由器的方法，ControllerAutoRoute控制器调用ControllerInjectAutoRoute方法注入。
func (ctl *ControllerAutoRoute) Inject(controller Controller, router Router) error {
	return ControllerInjectAutoRoute(controller, router)
}

// defaultGetViewTemplate 通过控制器名称和方法名称获得模板路径,如果文件不存在一般Render时会err显示路径。
//
// 格式: views/controller/%s/%s.html
//
// MyUserController Index => views/controller/my/user/index.html
func defaultGetViewTemplate(cname string, method string) string {
	if controllerHasSuffix(cname) {
		cname = cname[:len(cname)-10]
	}
	names := splitName(cname)
	for i := range names {
		names[i] = strings.ToLower(names[i])
	}
	return fmt.Sprintf("views/controller/%s/%s.html", strings.Join(names, "/"), strings.ToLower(method))
}

// Init 实现控制器初始方法。
func (ctl *ControllerView) Init(ctx Context) error {
	ctl.Context = ctx
	ctl.Data = make(map[string]interface{})
	return nil
}

// Release 实现控制器释放方法。
func (ctl *ControllerView) Release(Context) error {
	if ctl.Response().Size() == 0 && len(ctl.Data) != 0 {
		return ctl.Render(ctl.Data)
	}
	return nil
}

// Inject 方法实现控制器注入到路由器的方法，ControllerView控制器调用ControllerInjectStateful方法注入。
//
// ControllerView控制器在Release时，如果未写入数据会自动写入数据。
func (ctl *ControllerView) Inject(controller Controller, router Router) error {
	return ControllerInjectStateful(controller, router)
}

// GetRouteParam 方法返回路由的参数，View控制器会附加模板信息。
func (ctl *ControllerView) GetRouteParam(pkg, name, method string) string {
	return fmt.Sprintf("controllername=%s.%s controllermethod=%s template=%s", pkg, name, method, defaultGetViewTemplate(name, method))
}

// SetTemplate 方法设置模板文件路径。
func (ctl *ControllerView) SetTemplate(path string) {
	ctl.SetParam("template", path)
}

// NewControllerPoolStateful 函数创建一个基于sync.Pool的控制器池。
func NewControllerPoolStateful(controler Controller) ControllerPool {
	newfn := newControllerCloneFunc(controler)
	return &controllerPoolSync{
		Pool: sync.Pool{
			New: func() interface{} {
				return newfn()
			},
		},
	}
}

// newControllerCloneFunc 函数返回一个Controller克隆函数，复制控制器当前全部可导出数据。
func newControllerCloneFunc(controller Controller) func() interface{} {
	iType := reflect.TypeOf(controller).Elem()
	iValue := reflect.ValueOf(controller).Elem()
	// 获取控制器可导出非空属性
	var keys []int
	var vals []reflect.Value
	for i := 0; i < iValue.NumField(); i++ {
		field := iValue.Field(i)
		// go1.13 reflect.Value.IsZero
		if !checkValueIsZero(field) && field.CanSet() {
			keys = append(keys, i)
			vals = append(vals, iValue.Field(i))
		}
	}

	return func() interface{} {
		iValue := reflect.New(iType)
		for i, key := range keys {
			iValue.Elem().Field(key).Set(vals[i])
		}
		return iValue.Interface()
	}
}

// Get 方法从sync.Pool Get一个控制器。
func (pool *controllerPoolSync) Get() Controller {
	return pool.Pool.Get().(Controller)

}

// Put 方法将控制器放回到sync.Pool
func (pool *controllerPoolSync) Put(controller Controller) {
	pool.Pool.Put(controller)
}

// NewControllerPoolSingleton 函数创建一个单例对象的控制器池，每次都返回固定的单例控制器对象。
func NewControllerPoolSingleton(controler Controller) ControllerPool {
	return &controllerPoolSingleton{controler}
}

// Get 方法返回单例控制器对象。
func (pool *controllerPoolSingleton) Get() Controller {
	return pool.Controller

}

// Put 方法是空函数内容，不需要将单例控制器回收。
func (pool *controllerPoolSingleton) Put(Controller) {
	// Do nothing because controllerPoolSingleton not put data.
}

// NewExtendController 函数将控制器转换成HandlerFunc，需要提供控制器处理函数。
func NewExtendController(name string, pool ControllerPool, fn func(Context, Controller)) HandlerFunc {
	h := func(ctx Context) {
		controller := pool.Get()
		err := controller.Init(ctx)
		if err != nil {
			ctx.Fatal(err)
			return
		}

		fn(ctx, controller)

		err = controller.Release(ctx)
		if err != nil {
			ctx.Fatal(err)
		}
		pool.Put(controller)
	}
	SetHandlerFuncName(h, name)
	return h
}

// NewExtendControllerFunc 函数处理func()类型的控制器方法调用。
func NewExtendControllerFunc(ef ControllerFuncExtend) HandlerFunc {
	_, ok := reflect.ValueOf(ef.Controller).Method(ef.Index).Interface().(func())
	if !ok {
		return nil
	}

	index := ef.Index
	return NewExtendController(ef.Name, ef.Pool, func(_ Context, ctl Controller) {
		reflect.ValueOf(ctl).Method(index).Call(nil)
	})
}

// NewExtendControllerFuncRender 函数处理func() interface{}类型的控制器方法调用。
func NewExtendControllerFuncRender(ef ControllerFuncExtend) HandlerFunc {
	_, ok := reflect.ValueOf(ef.Controller).Method(ef.Index).Interface().(func() interface{})
	if !ok {
		return nil
	}

	index := ef.Index
	return NewExtendController(ef.Name, ef.Pool, func(ctx Context, ctl Controller) {
		data := reflect.ValueOf(ctl).Method(index).Call(nil)[0].Interface()
		if data != nil && ctx.Response().Size() == 0 {
			err := ctx.Render(data)
			if err != nil {
				ctx.Fatal(err)
			}
		}
	})
}

// NewExtendControllerFuncError 函数处理func() error类型的控制器方法调用。
func NewExtendControllerFuncError(ef ControllerFuncExtend) HandlerFunc {
	_, ok := reflect.ValueOf(ef.Controller).Method(ef.Index).Interface().(func() error)
	if !ok {
		return nil
	}

	index := ef.Index
	return NewExtendController(ef.Name, ef.Pool, func(ctx Context, ctl Controller) {
		ierr := reflect.ValueOf(ctl).Method(index).Call(nil)[0].Interface()
		if ierr != nil {
			ctx.Fatal(ierr)
		}
	})
}

// NewExtendControllerFuncRenderError 函数处理func() (interface{}, error)类型的控制器方法调用。
func NewExtendControllerFuncRenderError(ef ControllerFuncExtend) HandlerFunc {
	_, ok := reflect.ValueOf(ef.Controller).Method(ef.Index).Interface().(func() (interface{}, error))
	if !ok {
		return nil
	}

	index := ef.Index
	return NewExtendController(ef.Name, ef.Pool, func(ctx Context, ctl Controller) {
		data, err := reflect.ValueOf(ctl).Method(index).Interface().(func() (interface{}, error))()
		if err == nil && data != nil && ctx.Response().Size() == 0 {
			err = ctx.Render(data)
		}
		if err != nil {
			ctx.Fatal(err)
		}
	})
}

// NewExtendControllerFuncContext 函数处理func(Context)类型的控制器方法调用。
func NewExtendControllerFuncContext(ef ControllerFuncExtend) HandlerFunc {
	_, ok := reflect.ValueOf(ef.Controller).Method(ef.Index).Interface().(func(Context))
	if !ok {
		return nil
	}

	index := ef.Index
	return NewExtendController(ef.Name, ef.Pool, func(ctx Context, ctl Controller) {
		reflect.ValueOf(ctl).Method(index).Call([]reflect.Value{reflect.ValueOf(ctx)})
	})
}

// NewExtendControllerFuncContextRender 函数处理func(Context) interface{}类型的控制器方法调用。
func NewExtendControllerFuncContextRender(ef ControllerFuncExtend) HandlerFunc {
	_, ok := reflect.ValueOf(ef.Controller).Method(ef.Index).Interface().(func(Context) interface{})
	if !ok {
		return nil
	}

	index := ef.Index
	return NewExtendController(ef.Name, ef.Pool, func(ctx Context, ctl Controller) {
		data := reflect.ValueOf(ctl).Method(index).Call([]reflect.Value{reflect.ValueOf(ctx)})[0].Interface()
		if data != nil && ctx.Response().Size() == 0 {
			err := ctx.Render(data)
			if err != nil {
				ctx.Fatal(err)
			}
		}
	})
}

// NewExtendControllerFuncContextError 函数处理func(Context) error类型的控制器方法调用。
func NewExtendControllerFuncContextError(ef ControllerFuncExtend) HandlerFunc {
	_, ok := reflect.ValueOf(ef.Controller).Method(ef.Index).Interface().(func(Context) error)
	if !ok {
		return nil
	}

	index := ef.Index
	return NewExtendController(ef.Name, ef.Pool, func(ctx Context, ctl Controller) {
		ierr := reflect.ValueOf(ctl).Method(index).Call([]reflect.Value{reflect.ValueOf(ctx)})[0].Interface()
		if ierr != nil {
			ctx.Fatal(ierr)
		}
	})
}

// NewExtendControllerFuncContextRenderError 函数处理func(Context) (interface{}, error)类型的控制器方法调用。
func NewExtendControllerFuncContextRenderError(ef ControllerFuncExtend) HandlerFunc {
	_, ok := reflect.ValueOf(ef.Controller).Method(ef.Index).Interface().(func(Context) (interface{}, error))
	if !ok {
		return nil
	}

	index := ef.Index
	return NewExtendController(ef.Name, ef.Pool, func(ctx Context, ctl Controller) {
		data, err := reflect.ValueOf(ctl).Method(index).Interface().(func(Context) (interface{}, error))(ctx)
		if err == nil && data != nil && ctx.Response().Size() == 0 {
			err = ctx.Render(data)
		}
		if err != nil {
			ctx.Fatal(err)
		}
	})
}

// NewExtendControllerFuncMapString 函数处理func(map[string]interface{})类型的控制器方法调用。
func NewExtendControllerFuncMapString(ef ControllerFuncExtend) HandlerFunc {
	_, ok := reflect.ValueOf(ef.Controller).Method(ef.Index).Interface().(func(map[string]interface{}))
	if !ok {
		return nil
	}

	index := ef.Index
	return NewExtendController(ef.Name, ef.Pool, func(ctx Context, ctl Controller) {
		req := make(map[string]interface{})
		err := ctx.Bind(&req)
		if err != nil {
			ctx.Fatalf(ErrFormatControllerBind, err)
			return
		}
		reflect.ValueOf(ctl).Method(index).Call([]reflect.Value{reflect.ValueOf(req)})
	})
}

// NewExtendControllerFuncMapStringRender 函数处理func(map[string]interface{}) interface{}类型的控制器方法调用。
func NewExtendControllerFuncMapStringRender(ef ControllerFuncExtend) HandlerFunc {
	_, ok := reflect.ValueOf(ef.Controller).Method(ef.Index).Interface().(func(map[string]interface{}) interface{})
	if !ok {
		return nil
	}

	index := ef.Index
	return NewExtendController(ef.Name, ef.Pool, func(ctx Context, ctl Controller) {
		req := make(map[string]interface{})
		err := ctx.Bind(&req)
		if err != nil {
			ctx.Fatalf(ErrFormatControllerBind, err)
			return
		}
		data := reflect.ValueOf(ctl).Method(index).Call([]reflect.Value{reflect.ValueOf(req)})[0].Interface()
		if data != nil && ctx.Response().Size() == 0 {
			err := ctx.Render(data)
			if err != nil {
				ctx.Fatal(err)
			}
		}
	})
}

// NewExtendControllerFuncMapStringError 函数处理func(map[string]interface{}) error类型的控制器方法调用。
func NewExtendControllerFuncMapStringError(ef ControllerFuncExtend) HandlerFunc {
	_, ok := reflect.ValueOf(ef.Controller).Method(ef.Index).Interface().(func(map[string]interface{}) error)
	if !ok {
		return nil
	}

	index := ef.Index
	return NewExtendController(ef.Name, ef.Pool, func(ctx Context, ctl Controller) {
		req := make(map[string]interface{})
		err := ctx.Bind(&req)
		if err != nil {
			ctx.Fatalf(ErrFormatControllerBind, err)
			return
		}
		ierr := reflect.ValueOf(ctl).Method(index).Call([]reflect.Value{reflect.ValueOf(req)})[0].Interface()
		if ierr != nil {
			ctx.Fatal(ierr)
		}
	})
}

// NewExtendControllerFuncMapStringRenderError 函数处理func(map[string]interface{}) (interface{}, error)类型的控制器方法调用。
func NewExtendControllerFuncMapStringRenderError(ef ControllerFuncExtend) HandlerFunc {
	_, ok := reflect.ValueOf(ef.Controller).Method(ef.Index).Interface().(func(map[string]interface{}) (interface{}, error))
	if !ok {
		return nil
	}

	index := ef.Index
	return NewExtendController(ef.Name, ef.Pool, func(ctx Context, ctl Controller) {
		req := make(map[string]interface{})
		err := ctx.Bind(&req)
		if err != nil {
			ctx.Fatalf(ErrFormatControllerBind, err)
			return
		}
		data, err := reflect.ValueOf(ctl).Method(index).Interface().(func(map[string]interface{}) (interface{}, error))(req)
		if err == nil && data != nil && ctx.Response().Size() == 0 {
			err = ctx.Render(data)
		}
		if err != nil {
			ctx.Fatal(err)
		}
	})
}
