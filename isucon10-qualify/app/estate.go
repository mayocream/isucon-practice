package main

import (
	"context"
	"database/sql"
	"encoding/csv"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/go-redis/redis/v8"
	"github.com/gofiber/fiber/v2"
	geo "github.com/kellydunn/golang-geo"
	"github.com/spf13/cast"
)

func getEstateDetail(c *fiber.Ctx) error {
	id, err := strconv.Atoi(c.Params("id"))
	if err != nil {
		logger.Infof("Request parameter \"id\" parse error : %v", err)
		return c.SendStatus(http.StatusBadRequest)
	}

	val, err := redisClient.Get(context.Background(), CacheKeyEstateID+c.Params("id")).Result()
	if err != nil {
		if err == redis.Nil {
			logger.Infof("requested id's estate not found : %v", id)
			return c.SendStatus(http.StatusNotFound)
		}
		logger.Errorf("Failed to get the estate from id : %v", err)
		return c.SendStatus(http.StatusInternalServerError)
	}
	c.Set(fiber.HeaderContentType, fiber.MIMEApplicationJSON)
	return c.SendString(val)

	var estate Estate
	err = db.Get(&estate, "SELECT * FROM estate WHERE id = ?", id)
	if err != nil {
		if err == sql.ErrNoRows {
			logger.Infof("getEstateDetail estate id %v not found", id)
			return c.SendStatus(http.StatusNotFound)
		}
		logger.Errorf("Database Execution error : %v", err)
		return c.SendStatus(http.StatusInternalServerError)
	}

	return c.JSON(estate)
}

func postEstate(c *fiber.Ctx) error {
	header, err := c.FormFile("estates")
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

	tx, err := db.Begin()
	if err != nil {
		logger.Errorf("failed to begin tx: %v", err)
		return c.SendStatus(http.StatusInternalServerError)
	}
	defer tx.Rollback()

	rows := make([]*Estate, 0, len(records))
	pipe := redisClient.Pipeline()
	for _, record := range records {
		rm := RecordMapper{Record: record}
		id := rm.NextInt()
		name := rm.NextString()
		description := rm.NextString()
		thumbnail := rm.NextString()
		address := rm.NextString()
		latitude := rm.NextFloat()
		longitude := rm.NextFloat()
		rent := rm.NextInt()
		doorHeight := rm.NextInt()
		doorWidth := rm.NextInt()
		features := rm.NextString()
		popularity := rm.NextInt()
		if err := rm.Err(); err != nil {
			logger.Errorf("failed to read record: %v", err)
			return c.SendStatus(http.StatusBadRequest)
		}
		row := &Estate{
			ID:          cast.ToInt64(id),
			Thumbnail:   thumbnail,
			Name:        name,
			Description: description,
			Latitude:    latitude,
			Longitude:   longitude,
			Address:     address,
			Rent:        cast.ToInt64(rent),
			DoorHeight:  cast.ToInt64(doorHeight),
			DoorWidth:   cast.ToInt64(doorWidth),
			Features:    features,
			Popularity:  cast.ToInt64(popularity),
		}
		cacheRow(CacheKeyEstateID, row.ID, row, pipe)
		rows = append(rows, row)
	}

	go func() {
		for _, row := range rows {
			_, err := tx.Exec("INSERT INTO estate(id, name, description, thumbnail, address, latitude, longitude, rent, door_height, door_width, features, popularity) VALUES(?,?,?,?,?,?,?,?,?,?,?,?)", row.ID, row.Name, row.Description, row.Thumbnail, row.Address, row.Latitude, row.Longitude, row.Rent, row.DoorHeight, row.DoorWidth, row.Features, row.Popularity)
			if err != nil {
				logger.Errorf("failed to insert estate: %v", err)
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

// TODO inmemory search
func searchEstates(c *fiber.Ctx) error {
	conditions := make([]string, 0)
	params := make([]interface{}, 0)

	if c.Query("doorHeightRangeId") != "" {
		doorHeight, err := getRange(estateSearchCondition.DoorHeight, c.Query("doorHeightRangeId"))
		if err != nil {
			logger.Infof("doorHeightRangeID invalid, %v : %v", c.Query("doorHeightRangeId"), err)
			return c.SendStatus(http.StatusBadRequest)
		}

		if doorHeight.Min != -1 {
			conditions = append(conditions, "door_height >= ?")
			params = append(params, doorHeight.Min)
		}
		if doorHeight.Max != -1 {
			conditions = append(conditions, "door_height < ?")
			params = append(params, doorHeight.Max)
		}
	}

	if c.Query("doorWidthRangeId") != "" {
		doorWidth, err := getRange(estateSearchCondition.DoorWidth, c.Query("doorWidthRangeId"))
		if err != nil {
			logger.Infof("doorWidthRangeID invalid, %v : %v", c.Query("doorWidthRangeId"), err)
			return c.SendStatus(http.StatusBadRequest)
		}

		if doorWidth.Min != -1 {
			conditions = append(conditions, "door_width >= ?")
			params = append(params, doorWidth.Min)
		}
		if doorWidth.Max != -1 {
			conditions = append(conditions, "door_width < ?")
			params = append(params, doorWidth.Max)
		}
	}

	if c.Query("rentRangeId") != "" {
		estateRent, err := getRange(estateSearchCondition.Rent, c.Query("rentRangeId"))
		if err != nil {
			logger.Infof("rentRangeID invalid, %v : %v", c.Query("rentRangeId"), err)
			return c.SendStatus(http.StatusBadRequest)
		}

		if estateRent.Min != -1 {
			conditions = append(conditions, "rent >= ?")
			params = append(params, estateRent.Min)
		}
		if estateRent.Max != -1 {
			conditions = append(conditions, "rent < ?")
			params = append(params, estateRent.Max)
		}
	}

	if c.Query("features") != "" {
		for _, f := range strings.Split(c.Query("features"), ",") {
			// conditions = append(conditions, "features like '%?%'")
			conditions = append(conditions, "features like concat('%', ?, '%')")
			params = append(params, f)
		}
	}

	if len(conditions) == 0 {
		logger.Infof("searchEstates search condition not found")
		return c.SendStatus(http.StatusBadRequest)
	}

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

	searchQuery := "SELECT * FROM estate WHERE "
	countQuery := "SELECT COUNT(*) FROM estate WHERE "
	searchCondition := strings.Join(conditions, " AND ")
	limitOffset := " ORDER BY popularity_desc, id LIMIT ? OFFSET ?"

	var res EstateSearchResponse
	err = db.Get(&res.Count, countQuery+searchCondition, params...)
	if err != nil {
		logger.Errorf("searchEstates DB execution error : %v", err)
		return c.SendStatus(http.StatusInternalServerError)
	}

	estates := []Estate{}
	params = append(params, perPage, page*perPage)
	err = db.Select(&estates, searchQuery+searchCondition+limitOffset, params...)
	if err != nil {
		if err == sql.ErrNoRows {
			return c.JSON(EstateSearchResponse{Count: 0, Estates: []Estate{}})
		}
		logger.Errorf("searchEstates DB execution error : %v", err)
		return c.SendStatus(http.StatusInternalServerError)
	}

	res.Estates = estates

	return c.JSON(res)
}

// TODO cache
func getLowPricedEstate(c *fiber.Ctx) error {
	estates := make([]Estate, 0, Limit)
	query := `SELECT * FROM estate ORDER BY rent ASC, id ASC LIMIT ?`
	err := db.Select(&estates, query, Limit)
	if err != nil {
		if err == sql.ErrNoRows {
			logger.Error("getLowPricedEstate not found")
			return c.JSON(EstateListResponse{[]Estate{}})
		}
		logger.Errorf("getLowPricedEstate DB execution error : %v", err)
		return c.SendStatus(http.StatusInternalServerError)
	}

	return c.JSON(EstateListResponse{Estates: estates})
}

// TODO inmemory search
func searchRecommendedEstateWithChair(c *fiber.Ctx) error {
	id, err := strconv.Atoi(c.Params("id"))
	if err != nil {
		logger.Infof("Invalid format searchRecommendedEstateWithChair id : %v", err)
		return c.SendStatus(http.StatusBadRequest)
	}

	chair := Chair{}
	query := `SELECT * FROM chair WHERE id = ?`
	err = db.Get(&chair, query, id)
	if err != nil {
		if err == sql.ErrNoRows {
			logger.Infof("Requested chair id \"%v\" not found", id)
			return c.SendStatus(http.StatusBadRequest)
		}
		logger.Errorf("Database execution error : %v", err)
		return c.SendStatus(http.StatusInternalServerError)
	}

	var estates []Estate
	w := chair.Width
	h := chair.Height
	d := chair.Depth
	query = `SELECT * FROM estate WHERE (door_width >= ? AND door_height >= ?) OR (door_width >= ? AND door_height >= ?) OR (door_width >= ? AND door_height >= ?) OR (door_width >= ? AND door_height >= ?) OR (door_width >= ? AND door_height >= ?) OR (door_width >= ? AND door_height >= ?) ORDER BY popularity_desc, id LIMIT ?`
	err = db.Select(&estates, query, w, h, w, d, h, w, h, d, d, w, d, h, Limit)
	if err != nil {
		if err == sql.ErrNoRows {
			return c.JSON(EstateListResponse{[]Estate{}})
		}
		logger.Errorf("Database execution error : %v", err)
		return c.SendStatus(http.StatusInternalServerError)
	}

	return c.JSON(EstateListResponse{Estates: estates})
}

func searchEstateNazotte(c *fiber.Ctx) error {
	start := time.Now()

	coordinates := Coordinates{}
	err := c.BodyParser(&coordinates)
	if err != nil {
		logger.Infof("post search estate nazotte failed : %v", err)
		return c.SendStatus(http.StatusBadRequest)
	}

	if len(coordinates.Coordinates) == 0 {
		return c.SendStatus(http.StatusBadRequest)
	}

	// debug, TODO remove
	defer func() {
		duration := time.Since(start)
		logger.With("params", coordinates).Infof("request: post search estate nazotte, duration: %s", duration.String())
	}()

	b := coordinates.getBoundingBox()
	estatesInBoundingBox := []Estate{}
	query := `SELECT * FROM estate WHERE latitude <= ? AND latitude >= ? AND longitude <= ? AND longitude >= ? ORDER BY popularity_desc, id`
	err = db.Select(&estatesInBoundingBox, query, b.BottomRightCorner.Latitude, b.TopLeftCorner.Latitude, b.BottomRightCorner.Longitude, b.TopLeftCorner.Longitude)
	if err == sql.ErrNoRows {
		logger.Infof("select * from estate where latitude ...", err)
		return c.JSON(EstateSearchResponse{Count: 0, Estates: []Estate{}})
	} else if err != nil {
		logger.Errorf("database execution error : %v", err)
		return c.SendStatus(http.StatusInternalServerError)
	}

	estatesInPolygon := []Estate{}
	for _, estate := range estatesInBoundingBox {
		// if polygon contains point, we dont need SQL
		// ref: https://stackoverflow.com/questions/15618950/check-if-point-is-within-a-polygon
		p := geo.NewPoint(estate.Latitude, estate.Longitude)
		polygonPoints := make([]*geo.Point, 0, len(coordinates.Coordinates))
		for _, co := range coordinates.Coordinates {
			polygonPoints = append(polygonPoints, geo.NewPoint(co.Latitude, co.Longitude))
		}
		polygon := geo.NewPolygon(polygonPoints)
		if polygon.Contains(p) {
			estatesInPolygon = append(estatesInPolygon, estate)
		}
	}

	var re EstateSearchResponse
	re.Estates = []Estate{}
	if len(estatesInPolygon) > NazotteLimit {
		re.Estates = estatesInPolygon[:NazotteLimit]
	} else {
		re.Estates = estatesInPolygon
	}
	re.Count = int64(len(re.Estates))

	return c.JSON(re)
}

func postEstateRequestDocument(c *fiber.Ctx) error {
	params := new(buyParams)
	if err := c.BodyParser(params); err != nil {
		logger.Infof("post request document failed : %v", err)
		return c.SendStatus(http.StatusInternalServerError)
	}

	if params.Email == "" {
		logger.Info("post request document failed : email not found in request body")
		return c.SendStatus(http.StatusBadRequest)
	}

	id, err := strconv.Atoi(c.Params("id"))
	if err != nil {
		logger.Infof("post request document failed : %v", err)
		return c.SendStatus(http.StatusBadRequest)
	}

	estate := Estate{}
	query := `SELECT * FROM estate WHERE id = ?`
	err = db.Get(&estate, query, id)
	if err != nil {
		if err == sql.ErrNoRows {
			return c.SendStatus(http.StatusNotFound)
		}
		logger.Errorf("postEstateRequestDocument DB execution error : %v", err)
		return c.SendStatus(http.StatusInternalServerError)
	}

	return c.SendStatus(http.StatusOK)
}

func getEstateSearchCondition(c *fiber.Ctx) error {
	return c.JSON(estateSearchCondition)
}
