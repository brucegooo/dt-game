package common

import (
	"fmt"
	"runtime"
	"time"
)

func Printf(format string, v ...interface{}) {

	_, file, line, _ := runtime.Caller(1)
	loc := fmt.Sprintf("%s:%d", file, line)
	msg := fmt.Sprintf(format, v...)

	fmt.Println(time.Now().Format("2006-01-02 15:04:05.000"), "|", loc, "|", msg)
}

func Println(v ...interface{}) {

	_, file, line, _ := runtime.Caller(1)
	loc := fmt.Sprintf("%s:%d", file, line)
	msg := fmt.Sprint(v...)

	fmt.Println(time.Now().Format("2006-01-02 15:04:05.000"), "|", loc, "|", msg)
}
