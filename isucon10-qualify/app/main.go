package main

import (
    "context"
    "fmt"
    "net/http"
    _ "net/http/pprof"
    "os"
    "os/exec"
    "path/filepath"

    // "github.com/gchaincl/sqlhooks/v2"
    _ "github.com/go-sql-driver/mysql"
    "github.com/gofiber/fiber/v2"
    midLogger "github.com/gofiber/fiber/v2/middleware/logger"
)

func main() {
    // pprof
    go http.ListenAndServe("127.0.0.1:9090", nil)

    initLogger()

    s := fiber.New()
    routeRegister(s)

    if os.Getenv("ENV") == "dev" {
        logger.Info("use logger middleware")
        s.Use(midLogger.New())
    }

    mySQLConnectionData = NewMySQLConnectionEnv()

    var err error
    db, err = mySQLConnectionData.ConnectDB()
    if err != nil {
        logger.Fatalf("DB connection failed : %v", err)
    }
    // db.SetMaxOpenConns(10)
    defer db.Close()
    if err := db.Ping(); err != nil {
        logger.Errorf("DB Connect err: %s", err)
    }

    tablesCache()

    // Start server
    serverPort := fmt.Sprintf(":%v", getEnv("SERVER_PORT", "1323"))
    logger.Fatal(s.Listen(serverPort))
}

func initialize(c *fiber.Ctx) error {
    sqlDir := filepath.Join("..", "mysql", "db")
    paths := []string{
        filepath.Join(".", "0_Schema.sql"),
        filepath.Join(".", "0_Index.sql"),
        filepath.Join(sqlDir, "1_DummyEstateData.sql"),
        filepath.Join(sqlDir, "2_DummyChairData.sql"),
    }

    // Clean and initialize database
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
    if err := redisClient.FlushAll(context.Background()).Err(); err != nil {
        logger.Errorf("redis flush err: %s", err)
    }

    // create redis cache
    tablesCache()

    return c.JSON(InitializeResponse{
        Language: "go",
    })
}

