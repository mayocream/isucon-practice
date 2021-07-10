package main

import (
	"database/sql"
	"encoding/csv"
	"fmt"
	"net/http"
	_ "net/http/pprof"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	// "github.com/gchaincl/sqlhooks/v2"
	_ "github.com/go-sql-driver/mysql"
	"github.com/gofiber/fiber/v2"
	"github.com/kellydunn/golang-geo"
)

// TODO debug remove
// func init() {
//     sql.Register("mysql", sqlhooks.Wrap(&mysql.MySQLDriver{}, &Hooks{}))
// }

func main() {
    // pprof
    go http.ListenAndServe("127.0.0.1:9090", nil)

    initLogger()

    s := fiber.New()
    routeRegister(s)

    mySQLConnectionData = NewMySQLConnectionEnv()

    var err error
    db, err = mySQLConnectionData.ConnectDB()
    if err != nil {
        logger.Fatalf("DB connection failed : %v", err)
    }
    db.SetMaxOpenConns(10)
    defer db.Close()

    // Start server
    serverPort := fmt.Sprintf(":%v", getEnv("SERVER_PORT", "1323"))
    logger.Fatal(s.Listen(serverPort))
}

func initialize(c *fiber.Ctx) error {
    sqlDir := filepath.Join("..", "mysql", "db")
    paths := []string{
        filepath.Join(sqlDir, "0_Schema.sql"),
        filepath.Join(".", "0_Index.sql"),
        filepath.Join(sqlDir, "1_DummyEstateData.sql"),
        filepath.Join(sqlDir, "2_DummyChairData.sql"),
    }

    for _, p := range paths {
        sqlFile, _ := filepath.Abs(p)
        cmdStr := fmt.Sprintf("mysql -h %v -u %v -p%v -P %v %v < %v",
            mySQLConnectionData.Host,
            mySQLConnectionData.User,
            mySQLConnectionData.Password,
            mySQLConnectionData.Port,
            mySQLConnectionData.DBName,
            sqlFile,
        )
        if err := exec.Command("bash", "-c", cmdStr).Run(); err != nil {
            logger.Errorf("Initialize script error : %v", err)
            return c.SendStatus(http.StatusInternalServerError)
        }
    }

    return c.JSON(InitializeResponse{
        Language: "go",
    })
}

// TODO cache
func getChairDetail(c *fiber.Ctx) error {
    id, err := strconv.Atoi(c.Params("id"))
    if err != nil {
        logger.Errorf("Request parameter \"id\" parse error : %v", err)
        return c.SendStatus(http.StatusBadRequest)
    }

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

// TODO cache into memory
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

    tx, err := db.Begin()
    if err != nil {
        logger.Errorf("failed to begin tx: %v", err)
        return c.SendStatus(http.StatusInternalServerError)
    }
    defer tx.Rollback()
    for _, row := range records {
        rm := RecordMapper{Record: row}
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
        _, err := tx.Exec("INSERT INTO chair(id, name, description, thumbnail, price, height, width, depth, color, features, kind, popularity, stock) VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?)", id, name, description, thumbnail, price, height, width, depth, color, features, kind, popularity, stock)
        if err != nil {
            logger.Errorf("failed to insert chair: %v", err)
            return c.SendStatus(http.StatusInternalServerError)
        }
    }
    if err := tx.Commit(); err != nil {
        logger.Errorf("failed to commit tx: %v", err)
        return c.SendStatus(http.StatusInternalServerError)
    }
    return c.SendStatus(http.StatusCreated)
}

func searchChairs(c *fiber.Ctx) error {
    conditions := make([]string, 0)
    params := make([]interface{}, 0)

    if c.Query("priceRangeId") != "" {
        chairPrice, err := getRange(chairSearchCondition.Price, c.Query("priceRangeId"))
        if err != nil {
            logger.Infof("priceRangeID invalid, %v : %v", c.Query("priceRangeId"), err)
            return c.SendStatus(http.StatusBadRequest)
        }

        if chairPrice.Min != -1 {
            conditions = append(conditions, "price >= ?")
            params = append(params, chairPrice.Min)
        }
        if chairPrice.Max != -1 {
            conditions = append(conditions, "price < ?")
            params = append(params, chairPrice.Max)
        }
    }

    if c.Query("heightRangeId") != "" {
        chairHeight, err := getRange(chairSearchCondition.Height, c.Query("heightRangeId"))
        if err != nil {
            logger.Infof("heightRangeIf invalid, %v : %v", c.Query("heightRangeId"), err)
            return c.SendStatus(http.StatusBadRequest)
        }

        if chairHeight.Min != -1 {
            conditions = append(conditions, "height >= ?")
            params = append(params, chairHeight.Min)
        }
        if chairHeight.Max != -1 {
            conditions = append(conditions, "height < ?")
            params = append(params, chairHeight.Max)
        }
    }

    if c.Query("widthRangeId") != "" {
        chairWidth, err := getRange(chairSearchCondition.Width, c.Query("widthRangeId"))
        if err != nil {
            logger.Infof("widthRangeID invalid, %v : %v", c.Query("widthRangeId"), err)
            return c.SendStatus(http.StatusBadRequest)
        }

        if chairWidth.Min != -1 {
            conditions = append(conditions, "width >= ?")
            params = append(params, chairWidth.Min)
        }
        if chairWidth.Max != -1 {
            conditions = append(conditions, "width < ?")
            params = append(params, chairWidth.Max)
        }
    }

    if c.Query("depthRangeId") != "" {
        chairDepth, err := getRange(chairSearchCondition.Depth, c.Query("depthRangeId"))
        if err != nil {
            logger.Infof("depthRangeId invalid, %v : %v", c.Query("depthRangeId"), err)
            return c.SendStatus(http.StatusBadRequest)
        }

        if chairDepth.Min != -1 {
            conditions = append(conditions, "depth >= ?")
            params = append(params, chairDepth.Min)
        }
        if chairDepth.Max != -1 {
            conditions = append(conditions, "depth < ?")
            params = append(params, chairDepth.Max)
        }
    }

    if c.Query("kind") != "" {
        conditions = append(conditions, "kind = ?")
        params = append(params, c.Query("kind"))
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
    limitOffset := " ORDER BY popularity DESC, id ASC LIMIT ? OFFSET ?"

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

func buyChair(c *fiber.Ctx) error {
    m := make(map[string]interface{})
    if err := c.BodyParser(&m); err != nil {
        logger.Infof("post buy chair failed : %v", err)
        return c.SendStatus(http.StatusInternalServerError)
    }

    _, ok := m["email"].(string)
    if !ok {
        logger.Info("post buy chair failed : email not found in request body")
        return c.SendStatus(http.StatusBadRequest)
    }

    id, err := strconv.Atoi(c.Params("id"))
    if err != nil {
        logger.Infof("post buy chair failed : %v", err)
        return c.SendStatus(http.StatusBadRequest)
    }

    tx, err := db.Beginx()
    if err != nil {
        logger.Errorf("failed to create transaction : %v", err)
        return c.SendStatus(http.StatusInternalServerError)
    }
    defer tx.Rollback()

    var chair Chair
    err = tx.QueryRowx("SELECT * FROM chair WHERE id = ? AND stock > 0 FOR UPDATE", id).StructScan(&chair)
    if err != nil {
        if err == sql.ErrNoRows {
            logger.Infof("buyChair chair id \"%v\" not found", id)
            return c.SendStatus(http.StatusNotFound)
        }
        logger.Errorf("DB Execution Error: on getting a chair by id : %v", err)
        return c.SendStatus(http.StatusInternalServerError)
    }

    _, err = tx.Exec("UPDATE chair SET stock = stock - 1 WHERE id = ?", id)
    if err != nil {
        logger.Errorf("chair stock update failed : %v", err)
        return c.SendStatus(http.StatusInternalServerError)
    }

    err = tx.Commit()
    if err != nil {
        logger.Errorf("transaction commit error : %v", err)
        return c.SendStatus(http.StatusInternalServerError)
    }

    return c.SendStatus(http.StatusOK)
}

func getChairSearchCondition(c *fiber.Ctx) error {
    return c.JSON(chairSearchCondition)
}

func getLowPricedChair(c *fiber.Ctx) error {
    var chairs []Chair
    query := `SELECT * FROM chair WHERE stock > 0 ORDER BY price ASC, id ASC LIMIT ?`
    err := db.Select(&chairs, query, Limit)
    if err != nil {
        if err == sql.ErrNoRows {
            logger.Error("getLowPricedChair not found")
            return c.JSON(ChairListResponse{[]Chair{}})
        }
        logger.Errorf("getLowPricedChair DB execution error : %v", err)
        return c.SendStatus(http.StatusInternalServerError)
    }

    return c.JSON(ChairListResponse{Chairs: chairs})
}

func getEstateDetail(c *fiber.Ctx) error {
    id, err := strconv.Atoi(c.Params("id"))
    if err != nil {
        logger.Infof("Request parameter \"id\" parse error : %v", err)
        return c.SendStatus(http.StatusBadRequest)
    }

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

func getRange(cond RangeCondition, rangeID string) (*Range, error) {
    RangeIndex, err := strconv.Atoi(rangeID)
    if err != nil {
        return nil, err
    }

    if RangeIndex < 0 || len(cond.Ranges) <= RangeIndex {
        return nil, fmt.Errorf("Unexpected Range ID")
    }

    return cond.Ranges[RangeIndex], nil
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
    for _, row := range records {
        rm := RecordMapper{Record: row}
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
        _, err := tx.Exec("INSERT INTO estate(id, name, description, thumbnail, address, latitude, longitude, rent, door_height, door_width, features, popularity) VALUES(?,?,?,?,?,?,?,?,?,?,?,?)", id, name, description, thumbnail, address, latitude, longitude, rent, doorHeight, doorWidth, features, popularity)
        if err != nil {
            logger.Errorf("failed to insert estate: %v", err)
            return c.SendStatus(http.StatusInternalServerError)
        }
    }
    if err := tx.Commit(); err != nil {
        logger.Errorf("failed to commit tx: %v", err)
        return c.SendStatus(http.StatusInternalServerError)
    }
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
    limitOffset := " ORDER BY popularity DESC, id ASC LIMIT ? OFFSET ?"

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
    query = `SELECT * FROM estate WHERE (door_width >= ? AND door_height >= ?) OR (door_width >= ? AND door_height >= ?) OR (door_width >= ? AND door_height >= ?) OR (door_width >= ? AND door_height >= ?) OR (door_width >= ? AND door_height >= ?) OR (door_width >= ? AND door_height >= ?) ORDER BY popularity DESC, id ASC LIMIT ?`
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
    query := `SELECT * FROM estate WHERE latitude <= ? AND latitude >= ? AND longitude <= ? AND longitude >= ? ORDER BY popularity DESC, id ASC`
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

        // validatedEstate := Estate{}
        // point := fmt.Sprintf("'POINT(%f %f)'", estate.Latitude, estate.Longitude)
        // query := fmt.Sprintf(`SELECT * FROM estate WHERE id = ? AND ST_Contains(ST_PolygonFromText(%s), ST_GeomFromText(%s))`, coordinates.coordinatesToText(), point)
        // err = db.Get(&validatedEstate, query, estate.ID)
        // if err != nil {
        // 	if err == sql.ErrNoRows {
        // 		continue
        // 	} else {
        // 		logger.Errorf("db access is failed on executing validate if estate is in polygon : %v", err)
        // 		return c.SendStatus(http.StatusInternalServerError)
        // 	}
        // } else {
        // 	estatesInPolygon = append(estatesInPolygon, validatedEstate)
        // }
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
    m := make(map[string]interface{})
    if err := c.BodyParser(&m); err != nil {
        logger.Infof("post request document failed : %v", err)
        return c.SendStatus(http.StatusInternalServerError)
    }

    _, ok := m["email"].(string)
    if !ok {
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

func (cs Coordinates) getBoundingBox() BoundingBox {
    coordinates := cs.Coordinates
    boundingBox := BoundingBox{
        TopLeftCorner: Coordinate{
            Latitude: coordinates[0].Latitude, Longitude: coordinates[0].Longitude,
        },
        BottomRightCorner: Coordinate{
            Latitude: coordinates[0].Latitude, Longitude: coordinates[0].Longitude,
        },
    }
    for _, coordinate := range coordinates {
        if boundingBox.TopLeftCorner.Latitude > coordinate.Latitude {
            boundingBox.TopLeftCorner.Latitude = coordinate.Latitude
        }
        if boundingBox.TopLeftCorner.Longitude > coordinate.Longitude {
            boundingBox.TopLeftCorner.Longitude = coordinate.Longitude
        }

        if boundingBox.BottomRightCorner.Latitude < coordinate.Latitude {
            boundingBox.BottomRightCorner.Latitude = coordinate.Latitude
        }
        if boundingBox.BottomRightCorner.Longitude < coordinate.Longitude {
            boundingBox.BottomRightCorner.Longitude = coordinate.Longitude
        }
    }
    return boundingBox
}

func (cs Coordinates) coordinatesToText() string {
    points := make([]string, 0, len(cs.Coordinates))
    for _, c := range cs.Coordinates {
        points = append(points, fmt.Sprintf("%f %f", c.Latitude, c.Longitude))
    }
    return fmt.Sprintf("'POLYGON((%s))'", strings.Join(points, ","))
}
