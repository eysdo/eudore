package eudore

/*
Logger

Logger定义通用日志处理接口

文件: logger.go loggerstd.go
*/

import (
	"fmt"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"
)

// LoggerLevel 定义日志级别
type LoggerLevel int32

// Fields 定义多个日志属性
type Fields map[string]interface{}

// Logout 日志输出接口
type Logout interface {
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
}

// Logger 定义日志处理器定义
type Logger interface {
	Logout
	Sync() error
	SetLevel(LoggerLevel)
}

// loggerInitHandler 定义初始日志处理器必要接口，使用新日志处理器处理当前记录的全部日志。
type loggerInitHandler interface {
	NextHandler(Logger)
}

// LoggerInit the initial log processor only records the log. After setting the log processor,
// it will forward the log of the current record to the new log processor for processing the log generated before the program is initialized.
//
// LoggerInit 初始日志处理器仅记录日志，再设置日志处理器后，
// 会将当前记录的日志交给新日志处理器处理，用于处理程序初始化之前产生的日志。
type LoggerInit struct {
	level LoggerLevel
	mu    sync.Mutex
	data  []*entryInit
}
type entryInit struct {
	logger  *LoggerInit
	Level   LoggerLevel `json:"level"`
	Fields  Fields      `json:"fields,omitempty"`
	Time    time.Time   `json:"time"`
	Message string      `json:"message,omitempty"`
}

// 定义日志级别
const (
	LogDebug LoggerLevel = iota
	LogInfo
	LogWarning
	LogError
	LogFatal
	LogClose
)

// NewLoggerInit 函数创建一个初始化日志处理器。
func NewLoggerInit() Logger {
	return &LoggerInit{}
}

func (l *LoggerInit) newEntry() *entryInit {
	return &entryInit{logger: l, Time: time.Now()}
}

func (l *LoggerInit) putEntry(entry *entryInit) {
	if entry.Level >= l.level {
		l.mu.Lock()
		l.data = append(l.data, entry)
		l.mu.Unlock()
	}
}

// NextHandler 方法实现loggerInitHandler接口。
func (l *LoggerInit) NextHandler(logger Logger) {
	for _, e := range l.data {
		switch e.Level {
		case LogDebug:
			logger.WithFields(e.Fields).WithField("time", e.Time).Debug(e.Message)
		case LogInfo:
			logger.WithFields(e.Fields).WithField("time", e.Time).Info(e.Message)
		case LogWarning:
			logger.WithFields(e.Fields).WithField("time", e.Time).Warning(e.Message)
		case LogError:
			logger.WithFields(e.Fields).WithField("time", e.Time).Error(e.Message)
		case LogFatal:
			logger.WithFields(e.Fields).WithField("time", e.Time).Fatal(e.Message)
		}
	}
	l.data = l.data[0:0]
}

// SetLevel 方法设置日志处理级别。
func (l *LoggerInit) SetLevel(level LoggerLevel) {
	l.level = level
}

// Sync 方法
func (l *LoggerInit) Sync() error {
	return nil
}

// WithField 方法给日志新增一个属性。
func (l *LoggerInit) WithField(key string, value interface{}) Logout {
	return l.newEntry().WithField(key, value)
}

// WithFields 方法给日志新增多个属性。
func (l *LoggerInit) WithFields(fields Fields) Logout {
	return l.newEntry().WithFields(fields)
}

// Debug 方法输出Debug级别日志。
func (l *LoggerInit) Debug(args ...interface{}) {
	l.newEntry().Debug(args...)
}

// Info 方法输出Info级别日志。
func (l *LoggerInit) Info(args ...interface{}) {
	l.newEntry().Info(args...)
}

// Warning 方法输出Warning级别日志。
func (l *LoggerInit) Warning(args ...interface{}) {
	l.newEntry().Warning(args...)
}

// Error 方法输出Error级别日志。
func (l *LoggerInit) Error(args ...interface{}) {
	l.newEntry().Error(args...)
}

// Fatal 方法输出Fatal级别日志。
func (l *LoggerInit) Fatal(args ...interface{}) {
	l.newEntry().Fatal(args...)
}

// Debugf 方法格式化输出Debug级别日志。
func (l *LoggerInit) Debugf(format string, args ...interface{}) {
	l.newEntry().Debugf(format, args...)
}

// Infof 方法格式化输出Info级别日志。
func (l *LoggerInit) Infof(format string, args ...interface{}) {
	l.newEntry().Infof(format, args...)
}

// Warningf 方法格式化输出Warning级别日志。
func (l *LoggerInit) Warningf(format string, args ...interface{}) {
	l.newEntry().Warningf(format, args...)
}

// Errorf 方法格式化输出Error级别日志。
func (l *LoggerInit) Errorf(format string, args ...interface{}) {
	l.newEntry().Errorf(format, args...)
}

// Fatalf 方法格式化输出Fatal级别日志。
func (l *LoggerInit) Fatalf(format string, args ...interface{}) {
	l.newEntry().Fatalf(format, args...)
}

// Debug 方法输出Debug级别日志。
func (e *entryInit) Debug(args ...interface{}) {
	e.Level = 0
	e.Message = fmt.Sprintln(args...)
	e.Message = e.Message[:len(e.Message)-1]
	e.logger.putEntry(e)
}

// Info 方法输出Info级别日志。
func (e *entryInit) Info(args ...interface{}) {
	e.Level = 1
	e.Message = fmt.Sprintln(args...)
	e.Message = e.Message[:len(e.Message)-1]
	e.logger.putEntry(e)
}

// Warning 方法输出Warning级别日志。
func (e *entryInit) Warning(args ...interface{}) {
	e.Level = 2
	e.Message = fmt.Sprintln(args...)
	e.Message = e.Message[:len(e.Message)-1]
	e.logger.putEntry(e)
}

// Error 方法输出Error级别日志。
func (e *entryInit) Error(args ...interface{}) {
	e.Level = 3
	e.Message = fmt.Sprintln(args...)
	e.Message = e.Message[:len(e.Message)-1]
	e.logger.putEntry(e)
}

// Fatal 方法输出Fatal级别日志。
func (e *entryInit) Fatal(args ...interface{}) {
	e.Level = 4
	e.Message = fmt.Sprintln(args...)
	e.Message = e.Message[:len(e.Message)-1]
	e.logger.putEntry(e)
}

// Debugf 方法格式化输出Debug级别日志。
func (e *entryInit) Debugf(format string, args ...interface{}) {
	e.Level = 0
	e.Message = fmt.Sprintf(format, args...)
	e.logger.putEntry(e)
}

// Infof 方法格式化输出Info级别日志。
func (e *entryInit) Infof(format string, args ...interface{}) {
	e.Level = 1
	e.Message = fmt.Sprintf(format, args...)
	e.logger.putEntry(e)
}

// Warningf 方法格式化输出Warning级别日志。
func (e *entryInit) Warningf(format string, args ...interface{}) {
	e.Level = 2
	e.Message = fmt.Sprintf(format, args...)
	e.logger.putEntry(e)
}

// Errorf 方法格式化输出Error级别日志。
func (e *entryInit) Errorf(format string, args ...interface{}) {
	e.Level = 3
	e.Message = fmt.Sprintf(format, args...)
	e.logger.putEntry(e)
}

// Fatalf 方法格式化输出Fatal级别日志。
func (e *entryInit) Fatalf(format string, args ...interface{}) {
	e.Level = 4
	e.Message = fmt.Sprintf(format, args...)
	e.logger.putEntry(e)
}

// WithField 方法给日志新增一个属性。
func (e *entryInit) WithField(key string, value interface{}) Logout {
	if e.Fields == nil {
		e.Fields = make(Fields)
	}
	e.Fields[key] = value
	return e
}

// WithFields 方法给日志新增多个属性。
func (e *entryInit) WithFields(fields Fields) Logout {
	e.Fields = fields
	return e
}

// String 方法实现ftm.Stringer接口，格式化输出日志级别。
func (l LoggerLevel) String() string {
	return LogLevelString[l]
}

// MarshalText 方法实现encoding.TextMarshaler接口，用于编码日志级别。
func (l LoggerLevel) MarshalText() (text []byte, err error) {
	text = []byte(l.String())
	return
}

// UnmarshalText 方法实现encoding.TextUnmarshaler接口，用于解码日志级别。
func (l *LoggerLevel) UnmarshalText(text []byte) error {
	str := strings.ToUpper(string(text))
	for i, s := range LogLevelString {
		if s == str {
			*l = LoggerLevel(i)
			return nil
		}
	}
	n, err := strconv.Atoi(str)
	fmt.Println(n, err)
	if err == nil && n < 5 && n > -1 {
		*l = LoggerLevel(n)
		return nil
	}
	return ErrLoggerLevelUnmarshalText
}

// NewPrintFunc 函数使用app创建一个输出函数。
//
// 如果第一个参数Fields类型，则调用WithFields方法。
//
// 如果参数是一个error则输出error级别日志，否在输出info级别日志。
func NewPrintFunc(app *App) func(...interface{}) {
	var log Logout = app
	return func(args ...interface{}) {
		fields, ok := args[0].(Fields)
		if ok {
			printLogout(log.WithFields(fields), args[1:])
		} else {
			printLogout(log, args)
		}
	}
}

func printLogout(log Logout, args []interface{}) {
	if len(args) == 1 {
		err, ok := args[0].(error)
		if ok {
			log.Error(err)
			return
		}
	}
	log.Info(args...)
}

// logFormatNameFileLine 函数获得调用的文件位置和函数名称。
//
// 文件位置会从第一个src后开始截取，处理gopath下文件位置。
func logFormatNameFileLine(depth int) (string, string, int) {
	var name string
	ptr, file, line, ok := runtime.Caller(depth)
	if !ok {
		file = "???"
		line = 1
	} else {
		// slash := strings.LastIndex(file, "/")
		slash := strings.Index(file, "src")
		if slash >= 0 {
			file = file[slash+4:]
		}
		name = runtime.FuncForPC(ptr).Name()
	}
	return name, file, line
}

func getPanicStakc(depth int) []string {
	pc := make([]uintptr, 20)
	n := runtime.Callers(depth, pc)
	if n == 0 {
		return nil
	}

	pc = pc[:n] // pass only valid pcs to runtime.CallersFrames
	frames := runtime.CallersFrames(pc)
	stack := make([]string, 0, 20)

	frame, more := frames.Next()
	for more {
		pos := strings.Index(frame.File, "src")
		if pos >= 0 {
			frame.File = frame.File[pos+4:]
		}
		pos = strings.LastIndex(frame.Function, "/")
		if pos >= 0 {
			frame.Function = frame.Function[pos+1:]
		}
		stack = append(stack, fmt.Sprintf("%s:%d %s", frame.File, frame.Line, frame.Function))

		frame, more = frames.Next()
	}
	return stack
}

func printEmpty(...interface{}) {
	// Do nothing because  not print message.
}
