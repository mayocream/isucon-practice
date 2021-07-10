package main

import (
	"os"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

var logger *zap.SugaredLogger

func initLogger() {
	conf := zap.NewProductionConfig()
	conf.OutputPaths = []string{"/var/log/isucon/app.log"}
	if os.Getenv("ENV") == "dev" {
		conf.OutputPaths = []string{"stderr"}
		conf.EncoderConfig = zap.NewDevelopmentEncoderConfig()
		conf.Encoding = "console"
	}
	conf.Level = zap.NewAtomicLevelAt(zapcore.InfoLevel)
	conf.EncoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder
	if os.Getenv("ENV") == "prod" {
		conf.Level = zap.NewAtomicLevelAt(zapcore.WarnLevel)
	}
	l, _ := conf.Build()
	logger = l.Sugar()
}