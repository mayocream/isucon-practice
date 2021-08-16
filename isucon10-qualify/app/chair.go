package main

import (
	"context"
	"database/sql"
	"encoding/csv"
	"net/http"
	"strconv"
	"strings"

	"github.com/go-redis/redis/v8"
	"github.com/gofiber/fiber/v2"
	jsoniter "github.com/json-iterator/go"
	"github.com/spf13/cast"
)

// optimized, get row
func getChairDetail(c *fiber.Ctx) error {
	id, err := strconv.Atoi(c.Params("id"))
	if err != nil {
		logger.Errorf("Request parameter \"id\" parse error : %v", err)
		return c.SendStatus(http.StatusBadRequest)
	}

	val, err := redisClient.Get(context.Background(), CacheKeyChairID+c.Params("id")).Result()
	if err != nil {
		if err == redis.Nil {
			logger.Infof("requested id's chair not found : %v", id)
			return c.SendStatus(http.StatusNotFound)
		}
		logger.Errorf("Failed to get the chair from id : %v", err)
		return c.SendStatus(http.StatusInternalServerError)
	}
	c.Set(fiber.HeaderContentType, fiber.MIMEApplicationJSON)
	return c.SendString(val)

	chair := Chair{}
	query := `SELECT * FROM chair WHERE id = ?`
	err = db.Get(&chair, query, id)
	if err != nil {
		if err == sql.ErrNoRows {
			logger.Infof("requested id's chair not found : %v", id)
			return c.SendStatus(http.StatusNotFound)
		}
		logger.Errorf("Failed to get the chair from id : %v", err)
		return c.SendStatus(http.StatusInternalServerError)
	} else if chair.Stock <= 0 {
		logger.Infof("requested id's chair is sold out : %v", id)
		return c.SendStatus(http.StatusNotFound)
	}

	return c.JSON(chair)
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

// TODO
func searchChairs(c *fiber.Ctx) error {
	conditions := make([]string, 0)
	params := make([]interface{}, 0)

	pipe := redisClient.Pipeline()

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

		pipe.ZRangeByScore(context.Background(), cacheKey("chair", "price"), &redis.ZRangeBy{
			Min: min,
			Max: "(" + max,
		})
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

		pipe.ZRangeByScore(context.Background(), cacheKey("chair", "height"), &redis.ZRangeBy{
			Min: min,
			Max: "(" + max,
		})
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

		pipe.ZRangeByScore(context.Background(), cacheKey("chair", "width"), &redis.ZRangeBy{
			Min: min,
			Max: "(" + max,
		})
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

		pipe.ZRangeByScore(context.Background(), cacheKey("chair", "depth"), &redis.ZRangeBy{
			Min: min,
			Max: "(" + max,
		})
	}

	if c.Query("kind") != "" {

	}

	if c.Query("color") != "" {
		conditions = append(conditions, "color = ?")
		params = append(params, c.Query("color"))
	}

	if c.Query("features") != "" {
		for _, f := range strings.Split(c.Query("features"), ",") {
			// conditions = append(conditions, "features LIKE '%?%'")
			conditions = append(conditions, "features LIKE CONCAT('%', ?, '%')")
			params = append(params, f)
		}
	}

	if len(conditions) == 0 {
		logger.Infof("Search condition not found")
		return c.SendStatus(http.StatusBadRequest)
	}

	conditions = append(conditions, "stock > 0")

	page, err := strconv.Atoi(c.Query("page"))
	if err != nil {
		logger.Infof("Invalid format page parameter : %v", err)
		return c.SendStatus(http.StatusBadRequest)
	}

	perPage, err := strconv.Atoi(c.Query("perPage"))
	if err != nil {
		logger.Infof("Invalid format perPage parameter : %v", err)
		return c.SendStatus(http.StatusBadRequest)
	}

	searchQuery := "SELECT * FROM chair WHERE "
	countQuery := "SELECT COUNT(*) FROM chair WHERE "
	searchCondition := strings.Join(conditions, " AND ")
	limitOffset := " ORDER BY popularity_desc, id LIMIT ? OFFSET ?"

	var res ChairSearchResponse
	err = db.Get(&res.Count, countQuery+searchCondition, params...)
	if err != nil {
		logger.Errorf("searchChairs DB execution error : %v", err)
		return c.SendStatus(http.StatusInternalServerError)
	}

	chairs := []Chair{}
	params = append(params, perPage, page*perPage)
	err = db.Select(&chairs, searchQuery+searchCondition+limitOffset, params...)
	if err != nil {
		if err == sql.ErrNoRows {
			return c.JSON(ChairSearchResponse{Count: 0, Chairs: []Chair{}})
		}
		logger.Errorf("searchChairs DB execution error : %v", err)
		return c.SendStatus(http.StatusInternalServerError)
	}

	res.Chairs = chairs

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

	for _, id := range ids {
		id = CacheKeyChairID + id
	}

	rowsResult, err := redisClient.MGet(context.Background(), ids...).Result()
	if err != nil {
		logger.Errorf("redis mget stock err: %s", err)
		return err
	}
	rows := cast.ToStringSlice(rowsResult)

	chairs := make([]Chair, 0, len(rows))
	for _, row := range rows {
		var chair Chair
		if err := jsoniter.UnmarshalFromString(row, &chair); err != nil {
			logger.Errorf("chair Unmarshal err: %s", err)
		}
		chairs = append(chairs, chair)
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
		logger.Infof("post buy chair failed : %v", err)
		return c.SendStatus(http.StatusInternalServerError)
	}

	if params.Email == "" {
		logger.Info("post buy chair failed : email not found in request body")
		return c.SendStatus(http.StatusBadRequest)
	}

	id, err := strconv.Atoi(c.Params("id"))
	if err != nil {
		logger.Infof("post buy chair failed : %v", err)
		return c.SendStatus(http.StatusBadRequest)
	}

	stock, err := redisClient.Get(context.Background(), cacheKey("chair", "stock", cast.ToString(id))).Int()
	if err != nil {
		if err == redis.Nil {
			return c.SendStatus(http.StatusBadRequest)
		}
		logger.Errorf("redis err: %s", err)
		return c.SendStatus(http.StatusInternalServerError)
	}

	if stock <= 0 {
		logger.Infof("chair stock <= 0, id: %v", id)
		return c.SendStatus(http.StatusBadRequest)
	}

	newStock, err := redisClient.Decr(context.Background(), cacheKey("chair", "stock", cast.ToString(id))).Result()
	if err != nil {
		logger.Errorf("chair stock decr err: %s, id: %v", err, id)
		return c.SendStatus(http.StatusBadRequest)
	}

	if newStock <= 0 {
		// remove redis cache
		pipe := redisClient.Pipeline()
		redisClient.SRem(context.Background(), cacheKey("chair", "color"), id)
		redisClient.SRem(context.Background(), cacheKey("chair", "kind"), id)
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
