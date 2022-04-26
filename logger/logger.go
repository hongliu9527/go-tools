/*
 * @Author: hongliu
 * @Date: 2022-04-26 09:34:48
 * @LastEditors: hongliu
 * @LastEditTime: 2022-04-26 09:52:23
 * @FilePath: \go-tools\logger\logger.go
 * @Description: 封装日志
 *
 * Copyright (c) 2022 by 洪流, All Rights Reserved.
 */

// 日志系统设计需求:
// 1. 支持日志信息输出到终端和日志文件(支持同时输出),默认终端输出
//    1.1 支持日志文件容量限制
//    1.2 支持日志文件切片(按天进行切分))
//    1.3 支持设置最大保留天数
//    1.4 保证系统升级时不会清除存在的日志
//    1.5 当剩余磁盘空间少于日志文件容量[可选]
//    1.6 终端输出时支持颜色
// 2. 保证系统升级时不会清除日志系统配置
// 3. 支持日志输出等级

package logger

import (
	"fmt"
	"path"
	"runtime"
	"strconv"
	"time"

	rotatelogs "github.com/lestrrat-go/file-rotatelogs"
	"github.com/rifflock/lfshook"
	"github.com/sirupsen/logrus"
)

// Logger 日志记录器类型封装
var logger *logrus.Logger

// fixFields 日志上报的固定信息域
var fixFields = make(logrus.Fields, 0)

// 模块初始化函数
func init() {
	logger = logrus.New()
	logger = logrus.New()
	logger.SetFormatter(&Formatter{
		TimestampFormat: "2006-01-02 15:04:05.000", // 设置时间戳格式
		NoFieldsColors:  false,                     // 设置单行所有内容都显示颜色
	})
}

// LogConfig 日志配置
type LogConfig struct {
	AppName                string        // 应用程序名称
	LogDir                 string        // 保存目录
	IsSaveToFile           bool          // 是否保存日志文件
	ConsoleLogLevel        string        // 终端日志等级
	FileLogLevel           string        // 文件日志等级
	RotationIntervalTime   time.Duration // 日志分割时间间隔
	MaxRotationRemainCount uint          // 日志分割文件个数
}

// logDefaultConfig 默认日志配置
var logDefaultConfig = LogConfig{
	AppName:                "App",     // 默认日志文件以应用程序名称的前缀
	LogDir:                 "./log",   // 默认日志保存目录为当前目录下log目录
	IsSaveToFile:           true,      // 默认开启日志文件保存功能
	ConsoleLogLevel:        "info",    // 默认设置日志命令行输出级别
	FileLogLevel:           "info",    // 默认设置日志文件输出级别
	RotationIntervalTime:   time.Hour, // 默认设置每隔1个小时切分一次日志文件
	MaxRotationRemainCount: 24,        // 默认设置只保存24个小时的日志内容
}

// ChinaClock 中国时区时钟
type ChinaClock struct{}

// Now 查询当前时间
func (t ChinaClock) Now() time.Time {
	return time.Now().UTC().Add(8 * time.Hour)
}

// newLogFileHook 创建日志文件相关的钩子
// 1. 支持日志文件分割
func newLogFileHook(logDir string, logLevel logrus.Level) logrus.Hook {
	writer, err := rotatelogs.New(
		logDir+"/"+logDefaultConfig.AppName+"_%Y-%m-%d_%H.log",
		rotatelogs.WithClock(ChinaClock{}),
		rotatelogs.WithRotationTime(logDefaultConfig.RotationIntervalTime),    // 设置日志分割的时间
		rotatelogs.WithRotationCount(logDefaultConfig.MaxRotationRemainCount), // 设置文件清理前最多保存的个数
		// rotatelogs.WithMaxAge(time.Hour*24),        // 设置文件清理前的最长保存时间(WithMaxAge和WithRotationCount二者只能设置一个)
	)

	if err != nil {
		logrus.Errorf("配置日志文件分割属性失败(%s)", err.Error())
	}

	writerMap := make(lfshook.WriterMap)
	levels := []logrus.Level{
		logrus.DebugLevel,
		logrus.InfoLevel,
		logrus.WarnLevel,
		logrus.ErrorLevel,
		logrus.FatalLevel,
		logrus.PanicLevel,
	}

	for _, level := range levels {
		if int(level) <= int(logLevel) {
			writerMap[level] = writer
		}
	}

	lfsHook := lfshook.NewHook(writerMap, &logrus.TextFormatter{
		DisableColors:   true,
		FullTimestamp:   true,
		TimestampFormat: "2006-01-02 15:04:05.000",
	})
	return lfsHook
}

// SetAppName 设置日志文件名以对应的应用程序名称为前缀
func SetAppName(appName string) {
	logDefaultConfig.AppName = appName
}

// SetLogFileDir 设置日志文件目录
func SetLogFileDir(dir string) error {
	logDefaultConfig.LogDir = dir
	return nil
}

// SetLogRotationTime 设置日志文件分割时长(NOTE: 最小有效时间间隔为小时)
func SetLogRotationTime(duration time.Duration) {
	logDefaultConfig.RotationIntervalTime = duration
}

// SetLogRotationMaxFileCount 设置日志文件分割最大个数
func SetLogRotationMaxFileCount(fileCount uint) {
	logDefaultConfig.MaxRotationRemainCount = fileCount
}

// SetFileLevel 设置日志文件输出等级
func SetFileLevel(logLevel string) {
	level, err := logrus.ParseLevel(logLevel)
	if err != nil {
		logrus.Errorf("输入的日志等级(%s)校验失败(%s)", logLevel, err.Error())
		return
	}

	// 创建日志文件相关的钩子替换掉默认钩子从而实现日志文件保存功能
	hooks := make(logrus.LevelHooks)
	hooks.Add(newLogFileHook(logDefaultConfig.LogDir, level))
	logger.ReplaceHooks(hooks)
}

// SetFixedFields 设置日志上报的固定信息域
func SetFixedFields(fields logrus.Fields) {
	fixFields = fields
}

// AddFixedField 增加日志上报的固定信息域
func AddFixedField(key string, value interface{}) {
	fixFields[key] = value
}

// SetConsoleLevel 设置日志终端输出等级
func SetConsoleLevel(logLevel string) {
	level, err := logrus.ParseLevel(logLevel)
	if err != nil {
		logrus.Errorf("输入的日志等级(%s)校验失败(%s)", logLevel, err.Error())
		return
	}

	if logger != nil {
		logger.SetLevel(level)
	}
}

// Debug 输出Debug信息
func Debug(format string, v ...interface{}) {
	_, filepath, line, ok := runtime.Caller(1)
	if ok {
		format = "[" + path.Base(filepath) + ":" + strconv.Itoa(line) + "] " + format
	}

	now := time.Now().UTC().Add(8 * time.Hour)
	if logger != nil {
		if len(v) == 0 {
			logger.WithTime(now).WithFields(fixFields).Debug(format)
		} else {
			logger.WithTime(now).WithFields(fixFields).Debugf(format, v...)
		}
	} else {
		fmt.Println("日志记录器未创建")
	}
}

// Info 输出Info信息
func Info(format string, v ...interface{}) {
	_, filepath, line, ok := runtime.Caller(1)
	if ok {
		format = "[" + path.Base(filepath) + ":" + strconv.Itoa(line) + "] " + format
	}

	now := time.Now().UTC().Add(8 * time.Hour)
	if logger != nil {
		if len(v) == 0 {
			logger.WithTime(now).WithFields(fixFields).Info(format)
		} else {
			logger.WithTime(now).WithFields(fixFields).Infof(format, v...)
		}
	} else {
		fmt.Println("日志记录器未创建")
	}
}

// Warning 输出Warning信息
func Warning(format string, v ...interface{}) {
	_, filepath, line, ok := runtime.Caller(1)
	if ok {
		format = "[" + path.Base(filepath) + ":" + strconv.Itoa(line) + "] " + format
	}

	now := time.Now().UTC().Add(8 * time.Hour)
	if logger != nil {
		if len(v) == 0 {
			logger.WithTime(now).WithFields(fixFields).Warn(format)
		} else {
			logger.WithTime(now).WithFields(fixFields).Warnf(format, v...)
		}
	} else {
		fmt.Println("日志记录器未创建")
	}
}

// Error 输出Error信息
func Error(format string, v ...interface{}) {
	_, filepath, line, ok := runtime.Caller(1)
	if ok {
		format = "[" + path.Base(filepath) + ":" + strconv.Itoa(line) + "] " + format
	}

	now := time.Now().UTC().Add(8 * time.Hour)
	if logger != nil {
		if len(v) == 0 {
			logger.WithTime(now).WithFields(fixFields).Error(format)
		} else {
			logger.WithTime(now).WithFields(fixFields).Errorf(format, v...)
		}
	} else {
		fmt.Println("日志记录器未创建")
	}
}

// Fatal 输出Fatal信息
func Fatal(format string, v ...interface{}) {
	_, filepath, line, ok := runtime.Caller(1)
	if ok {
		format = "[" + path.Base(filepath) + ":" + strconv.Itoa(line) + "] " + format
	}

	now := time.Now().UTC().Add(8 * time.Hour)
	if logger != nil {
		if len(v) == 0 {
			logger.WithTime(now).WithFields(fixFields).Fatal(format)
		} else {
			logger.WithTime(now).WithFields(fixFields).Fatalf(format, v...)
		}
	} else {
		fmt.Println("日志记录器未创建")
	}
}
