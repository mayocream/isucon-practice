package main

import (
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

var logger *zap.SugaredLogger

func initLogger() {
	conf := zap.NewProductionConfig()
	conf.OutputPaths = []string{"/var/log/isucon/app.log"}
	conf.Level = zap.NewAtomicLevelAt(zapcore.InfoLevel)
	conf.EncoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder
	l, _ := conf.Build()
	logger = l.Sugar()
}