package logger

import (
	"github.com/replicatedhq/kgrid/pkg/kgrid/grid/types"
)

type Logger interface {
	Silence()
	Verbose()
	Initialize()
	Finish()
	Debug(msg string, args ...interface{})
	Info(msg string, args ...interface{})
	ActionWithoutSpinner(msg string, args ...interface{})
	ChildActionWithoutSpinner(msg string, args ...interface{})
	ActionWithSpinner(msg string, args ...interface{})
	ChildActionWithSpinner(msg string, args ...interface{})
	FinishChildSpinner()
	FinishSpinner()
	FinishSpinnerWithError()
	Error(err error)
}

func NewLogger(loggerSpec types.LoggerSpec) Logger {
	if loggerSpec.Slack != nil {
		return NewSlackLogger(loggerSpec.Slack)
	}
	return NewTerminalLogger()
}
