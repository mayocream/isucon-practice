package main

import (
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

var logger *zap.SugaredLogger

func initLogger() {
	conf := zap.NewProductionConfig()
	conf.EncoderConfig = zap.NewDevelopmentEncoderConfig()
	conf.Encoding = "console"
	conf.Level = zap.NewAtomicLevelAt(zapcore.InfoLevel)
	l, _ := conf.Build()
	logger = l.Sugar()
}