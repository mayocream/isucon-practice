package main

import (
	"context"
	"os"
	"time"

	"github.com/go-redis/redis/v8"
	jsoniter "github.com/json-iterator/go"
	"github.com/spf13/cast"
)

// cache key
const (
	CacheKeyChairID = "chair:id:"
	CacheKeyEstateID = "estate:id:"
)

var redisClient *redis.Client

func initRedis() {
	addr := os.Getenv("REDIS_ADDR")
	if addr == "" {
		logger.Error("Redis addr env empty")
		return
	}

	rdb := redis.NewClient(&redis.Options{
		Addr: addr,
	})

	ctx, cancel := context.WithTimeout(context.Background(), 5 * time.Second) 
	defer cancel()
	if err := rdb.Ping(ctx); err != nil {
		logger.Errorf("Redis ping err: %s", err)
	}

	redisClient = rdb
}

func tablesCache() {
	{
		query := "SELECT * FROM chair"
		data := make([]Chair, 0, 3200)
		if err := db.Select(&data, query); err != nil {
			logger.Errorf("chair table query err: %s", err)
		}
	
		pipe := redisClient.Pipeline()
		for _, row := range data {
			cacheRow(CacheKeyChairID, row.ID, row, pipe)
		}
		if _, err := pipe.Exec(context.Background()); err != nil {
			logger.Errorf("redis cache chair err: %s", err)
		}
	}

	{
		query := "SELECT * FROM estate"
		data := make([]Estate, 0, 3200)
		if err := db.Select(&data, query); err != nil {
			logger.Errorf("estate table query err: %s", err)
		}
	
		pipe := redisClient.Pipeline()
		for _, row := range data {
			cacheRow(CacheKeyChairID, row.ID, row, pipe)
		}
		if _, err := pipe.Exec(context.Background()); err != nil {
			logger.Errorf("redis cache estate err: %s", err)
		}
	}
}

func cacheRow(prefix string, id interface{}, row interface{}, r redis.Pipeliner) {
	val, _ := jsoniter.MarshalToString(row)
	var err error
	if r != nil {
		err = r.Set(context.Background(), prefix + cast.ToString(id), val, 3600 * time.Second).Err()
	} else {
		err = redisClient.Set(context.Background(), prefix + cast.ToString(id), val, 3600 * time.Second).Err()
	}
	if err != nil {
		logger.Errorf("redis cache row err: %s", err)
	}
}
