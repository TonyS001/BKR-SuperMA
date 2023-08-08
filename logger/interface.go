// (C) 2016-2023 Ant Group Co.,Ltd.
// SPDX-License-Identifier: Apache-2.0

package logger

type Logger interface {
	Infof(format string, v ...interface{})
	Infoln(msg string)
	Debugf(format string, v ...interface{})
	Debugln(msg string)
	Warnf(format string, v ...interface{})
	Warnln(msg string)
	Error(msg string, err error)
	Errorln(msg string)
}
