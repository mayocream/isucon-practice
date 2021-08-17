package main

import (
	"context"
	"database/sql"
	"encoding/csv"
	"net/http"
	"sort"
	"strconv"
	"strings"

	"github.com/go-redis/redis/v8"
	"github.com/gofiber/fiber/v2"
	"github.com/spf13/cast"
	"github.com/thoas/go-funk"
)

// optimized, get row
func getChairDetail(c *fiber.Ctx) error {
	id := c.Params("id")
	if _, err := strconv.Atoi(id); err != nil {
		logger.Errorf("Request parameter \"id\" parse error : %v", err)
		return c.SendStatus(http.StatusBadRequest)
	}

	obj, err := getChairCache(id)
	if err != nil {
		return c.SendStatus(404)
	}
	return c.JSON(obj)
}

// optimized
// parse csv from input, cache to redis, store to db
func postChair(c *fiber.Ctx) error {
	header, err := c.FormFile("chairs")
	if err != nil {
		logger.Errorf("failed to get form file: %v", err)
		return c.SendStatus(http.StatusBadRequest)
	}
	f, err := header.Open()
	if err != nil {
		logger.Errorf("failed to open form file: %v", err)
		return c.SendStatus(http.StatusInternalServerError)
	}
	defer f.Close()
	records, err := csv.NewReader(f).ReadAll()
	if err != nil {
		logger.Errorf("failed to read csv: %v", err)
		return c.SendStatus(http.StatusInternalServerError)
	}

	// build rows
	rows := make([]*Chair, 0, len(records))
	pipe := redisClient.TxPipeline()
	for _, record := range records {
		rm := RecordMapper{Record: record}
		id := rm.NextInt()
		name := rm.NextString()
		description := rm.NextString()
		thumbnail := rm.NextString()
		price := rm.NextInt()
		height := rm.NextInt()
		width := rm.NextInt()
		depth := rm.NextInt()
		color := rm.NextString()
		features := rm.NextString()
		kind := rm.NextString()
		popularity := rm.NextInt()
		stock := rm.NextInt()
		if err := rm.Err(); err != nil {
			logger.Errorf("failed to read record: %v", err)
			return c.SendStatus(http.StatusBadRequest)
		}
		row := &Chair{
			ID:          cast.ToInt64(id),
			Name:        name,
			Description: description,
			Thumbnail:   thumbnail,
			Price:       cast.ToInt64(price),
			Height:      cast.ToInt64(height),
			Width:       cast.ToInt64(width),
			Depth:       cast.ToInt64(depth),
			Color:       color,
			Features:    features,
			Kind:        kind,
			Popularity:  cast.ToInt64(popularity),
			Stock:       cast.ToInt64(stock),
		}
		// cache
		cacheRow(CacheKeyChairID, row.ID, row, pipe)
		rows = append(rows, row)
	}

	if _, err := pipe.Exec(context.Background()); err != nil {
		logger.Errorf("redis cache chair err: %s", err)
		return err
	}

	// goroutine for SQL exec
	go func() {
		tx, err := db.Begin()
		if err != nil {
			logger.Errorf("failed to begin tx: %v", err)
			return
		}
		defer tx.Rollback()
		for _, row := range rows {
			_, err := tx.Exec("INSERT INTO chair(id, name, description, thumbnail, price, height, width, depth, color, features, kind, popularity, stock) VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?)", row.ID, row.Name, row.Description, row.Thumbnail, row.Price, row.Height, row.Width, row.Depth, row.Color, row.Features, row.Kind, row.Popularity, row.Stock)
			if err != nil {
				logger.Errorf("failed to insert chair: %v", err)
				return
			}
		}
		if err := tx.Commit(); err != nil {
			logger.Errorf("failed to commit tx: %v", err)
			return
		}
	}()

	return c.SendStatus(http.StatusCreated)
}

// optimized, TEST
func searchChairs(c *fiber.Ctx) error {
	pipe := redisClient.Pipeline()
	ctx := context.Background()

	cmds := make([]*redis.StringSliceCmd, 0)
	if c.Query("priceRangeId") != "" {
		chairPrice, err := getRange(chairSearchCondition.Price, c.Query("priceRangeId"))
		if err != nil {
			logger.Infof("priceRangeID invalid, %v : %v", c.Query("priceRangeId"), err)
			return c.SendStatus(http.StatusBadRequest)
		}

		min := "-inf"
		if chairPrice.Min != -1 {
			min = cast.ToString(chairPrice.Min)
		}
		max := "+inf"
		if chairPrice.Max != -1 {
			max = cast.ToString(chairPrice.Max)
		}

		cmd := pipe.ZRangeByScore(ctx, cacheKey("chair", "price"), &redis.ZRangeBy{
			Min: min,
			Max: "(" + max,
		})
		cmds = append(cmds, cmd)
	}

	if c.Query("heightRangeId") != "" {
		chairHeight, err := getRange(chairSearchCondition.Height, c.Query("heightRangeId"))
		if err != nil {
			logger.Infof("heightRangeIf invalid, %v : %v", c.Query("heightRangeId"), err)
			return c.SendStatus(http.StatusBadRequest)
		}

		min := "-inf"
		if chairHeight.Min != -1 {
			min = cast.ToString(chairHeight.Min)
		}
		max := "+inf"
		if chairHeight.Max != -1 {
			max = cast.ToString(chairHeight.Max)
		}

		cmd := pipe.ZRangeByScore(ctx, cacheKey("chair", "height"), &redis.ZRangeBy{
			Min: min,
			Max: "(" + max,
		})
		cmds = append(cmds, cmd)
	}

	if c.Query("widthRangeId") != "" {
		chairWidth, err := getRange(chairSearchCondition.Width, c.Query("widthRangeId"))
		if err != nil {
			logger.Infof("widthRangeID invalid, %v : %v", c.Query("widthRangeId"), err)
			return c.SendStatus(http.StatusBadRequest)
		}

		min := "-inf"
		if chairWidth.Min != -1 {
			min = cast.ToString(chairWidth.Min)
		}
		max := "+inf"
		if chairWidth.Max != -1 {
			max = cast.ToString(chairWidth.Max)
		}

		cmd := pipe.ZRangeByScore(ctx, cacheKey("chair", "width"), &redis.ZRangeBy{
			Min: min,
			Max: "(" + max,
		})
		cmds = append(cmds, cmd)
	}

	if c.Query("depthRangeId") != "" {
		chairDepth, err := getRange(chairSearchCondition.Depth, c.Query("depthRangeId"))
		if err != nil {
			logger.Infof("depthRangeId invalid, %v : %v", c.Query("depthRangeId"), err)
			return c.SendStatus(http.StatusBadRequest)
		}

		min := "-inf"
		if chairDepth.Min != -1 {
			min = cast.ToString(chairDepth.Min)
		}
		max := "+inf"
		if chairDepth.Max != -1 {
			max = cast.ToString(chairDepth.Max)
		}

		cmd := pipe.ZRangeByScore(ctx, cacheKey("chair", "depth"), &redis.ZRangeBy{
			Min: min,
			Max: "(" + max,
		})
		cmds = append(cmds, cmd)
	}

	if c.Query("kind") != "" {
		cmd := pipe.SMembers(ctx, cacheKey("chair", "kind", c.Query("kind")))
		cmds = append(cmds, cmd)
	}

	if c.Query("color") != "" {
		cmd := pipe.SMembers(ctx, cacheKey("chair", "color", c.Query("color")))
		cmds = append(cmds, cmd)
	}

	if c.Query("features") != "" {
		keys := make([]string, 0, 5)
		for _, feature := range strings.Split(c.Query("features"), ",") {
			keys = append(keys, cacheKey("chair", "features", featureID(feature)))
		}
		cmd := pipe.SInter(ctx, keys...)
		cmds = append(cmds, cmd)
	}

	if _, err := pipe.Exec(ctx); err != nil {
		return err
	}

	filterIds := make([]string, 0, 20)
	for _, cmd := range cmds {
		ids, err := cmd.Result()
		if err != nil {
			logger.Errorf("redis exec err: %s", err)
			return err
		}
		if len(ids) > 0 && len(filterIds) > 0 {
			filterIds = funk.IntersectString(ids, filterIds)
		}
		if len(filterIds) == 0 {
			filterIds = ids
		}
	}

	mgetResult, err := redisClient.MGet(ctx, filterIds...).Result()
	pops := cast.ToIntSlice(mgetResult)
	sort.SliceStable(filterIds, func(i, j int) bool {
		return pops[i] > pops[j]
	})

	page, err := strconv.Atoi(c.Query("page"))
	if err != nil {
		logger.Infof("Invalid format page parameter : %v", err)
		return c.SendStatus(http.StatusBadRequest)
	}

	limit, err := strconv.Atoi(c.Query("perPage"))
	if err != nil {
		logger.Infof("Invalid format perPage parameter : %v", err)
		return c.SendStatus(http.StatusBadRequest)
	}

	total := len(filterIds)
	idxStart, idxEnd := GetValidPagination(total, (page - 1) * limit, limit)
	filterIds = filterIds[idxStart:idxEnd]

	res := &ChairSearchResponse{
		Count: total,
	}
	objs := make([]*Chair, 0, len(filterIds))
	for _, id := range filterIds {
		obj, err := getChairCache(id)
		if err != nil {
			return err
		}
		objs = append(objs, obj)
	}
	res.Chairs = objs

	return c.JSON(res)
}

// don't need optimize
func getChairSearchCondition(c *fiber.Ctx) error {
	return c.JSON(chairSearchCondition)
}

// optimized, get 20 lowest-price chairs
func getLowPricedChair(c *fiber.Ctx) error {
	ids, err := redisClient.ZRange(context.Background(), cacheKey("chair", "price"), 0, 19).Result()
	if err != nil {
		logger.Errorf("redis get chair price err: %s", err)
		return err
	}

	for i, id := range ids {
		ids[i] = CacheKeyChairID + id
	}

	chairs := make([]*Chair, 0, len(ids))
	for _, id := range ids {
		obj, err := getChairCache(id)
		if err != nil {
			return err
		}
		chairs = append(chairs, obj)
	}

	return c.JSON(ChairListResponse{Chairs: chairs})
}

type buyParams struct {
	Email string `json:"email" form:"email"`
}

// optimized, reduce stock and store to DB
func buyChair(c *fiber.Ctx) error {
	params := new(buyParams)
	if err := c.BodyParser(params); err != nil {
		return c.SendStatus(http.StatusBadRequest)
	}

	if params.Email == "" {
		return c.SendStatus(http.StatusBadRequest)
	}

	id := c.Params("id")
	if id == "" {
		return c.SendStatus(http.StatusBadRequest)
	}

	if _, err := strconv.Atoi(id); err != nil {
		return c.SendStatus(http.StatusBadRequest)
	}

	ctx := context.Background()
	stock, err := redisClient.Get(ctx, cacheKey("chair", "stock", id)).Int()
	if err != nil {
		return c.SendStatus(http.StatusBadRequest)
	}

	if stock <= 0 {
		return c.SendStatus(http.StatusBadRequest)
	}

	newStock, err := redisClient.Decr(ctx, cacheKey("chair", "stock", id)).Result()
	if err != nil {
		logger.Errorf("chair stock decr err: %s, id: %v", err, id)
		return c.SendStatus(http.StatusBadRequest)
	}

	obj, err := getChairCache(id)
	if err != nil {
		return c.SendStatus(http.StatusBadRequest)
	}

	if newStock <= 0 {
		// remove redis cache
		pipe := redisClient.Pipeline()
		redisClient.SRem(context.Background(), cacheKey("chair", "color", obj.Color), id)
		redisClient.SRem(context.Background(), cacheKey("chair", "kind", obj.Kind), id)
		redisClient.ZRem(context.Background(), cacheKey("chair", "price"), id)
		redisClient.ZRem(context.Background(), cacheKey("chair", "height"), id)
		redisClient.ZRem(context.Background(), cacheKey("chair", "width"), id)
		redisClient.ZRem(context.Background(), cacheKey("chair", "depth"), id)

		if _, err := pipe.Exec(context.Background()); err != nil {
			logger.Errorf("redis exec err: %s", err)
		}
	}

	// goroutine for SQL exec
	go func() {
		tx, err := db.Beginx()
		if err != nil {
			logger.Errorf("failed to create transaction : %v", err)
			return
		}
		defer tx.Rollback()

		var chair Chair
		err = tx.QueryRowx("SELECT * FROM chair WHERE id = ? AND stock > 0 FOR UPDATE", id).StructScan(&chair)
		if err != nil {
			if err == sql.ErrNoRows {
				logger.Infof("buyChair chair id \"%v\" not found", id)
				return
			}
			logger.Errorf("DB Execution Error: on getting a chair by id : %v", err)
			return
		}

		_, err = tx.Exec("UPDATE chair SET stock = stock - 1 WHERE id = ?", id)
		if err != nil {
			logger.Errorf("chair stock update failed : %v", err)
			return
		}

		err = tx.Commit()
		if err != nil {
			logger.Errorf("transaction commit error : %v", err)
			return
		}
	}()

	return c.SendStatus(http.StatusOK)
}
