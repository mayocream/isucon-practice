package main

import (
	"context"
	"os"
	"strings"
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

func cacheKey(arr ...string) string {
    return strings.Join(arr, ":")
}

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
    // cache `chair` table
    {
        query := "SELECT * FROM chair"
        data := make([]*Chair, 0, 3200)
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

        if err := cacheChair(data...); err != nil {
            logger.Errorf("cache chair err: %s", err)
        }
    }

    // cache `estate` table
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

// cache a DB Row and encode in JSON
func cacheRow(prefix string, id interface{}, row interface{}, r redis.Pipeliner) {
    val, _ := jsoniter.MarshalToString(row)
    var err error
    if r != nil {
        err = r.Set(context.Background(), prefix + cast.ToString(id), val, 0).Err()
    } else {
        err = redisClient.Set(context.Background(), prefix + cast.ToString(id), val, 
        0).Err()
    }
    if err != nil {
        logger.Errorf("redis cache row err: %s", err)
    }
}

// fetch one from cache and decode JSON
func fetchCacheRow(prefix string, id interface{}, data interface{}) error {
    val, err := redisClient.Get(context.Background(), prefix + cast.ToString(id)).Result()
    if err != nil {
        return err
    }
    if err := jsoniter.UnmarshalFromString(val, data); err != nil {
        return err
    }
    return nil
}

// cache chair table, create indexes for searching
func cacheChair(arr ...*Chair) error {
    pipe := redisClient.Pipeline()
    for _, row := range arr {
        pipe.SAdd(context.Background(), cacheKey("chair", "color", row.Color), row.ID)
        pipe.SAdd(context.Background(), cacheKey("chair", "kind", row.Kind), row.ID)
        pipe.ZAdd(context.Background(), cacheKey("chair", "price"), &redis.Z{
            Score: float64(row.Price),
            Member: row.ID,
        })
        pipe.ZAdd(context.Background(), cacheKey("chair", "height"), &redis.Z{
            Score: float64(row.Height),
            Member: row.ID,
        })
        pipe.ZAdd(context.Background(), cacheKey("chair", "width"), &redis.Z{
            Score: float64(row.Width),
            Member: row.ID,
        })
        pipe.ZAdd(context.Background(), cacheKey("chair", "depth"), &redis.Z{
            Score: float64(row.Depth),
            Member: row.ID,
        })
        pipe.Set(context.Background(), cacheKey("chair", "popularity", cast.ToString(row.ID)), row.Popularity, 0)
        pipe.Set(context.Background(), cacheKey("chair", "stock", cast.ToString(row.ID)), row.Stock, 0)
    }
    if _, err := pipe.Exec(context.Background()); err != nil {
        logger.Errorf("redis cache chair err: %s", err)
    }
    return nil
}