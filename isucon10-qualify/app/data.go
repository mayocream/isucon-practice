package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"strconv"

	"github.com/jmoiron/sqlx"
)

// default conditions
const (
	Limit        = 20
	NazotteLimit = 50
)

var (
	db                    *sqlx.DB
	mySQLConnectionData   *MySQLConnectionEnv
	chairSearchCondition  ChairSearchCondition
	estateSearchCondition EstateSearchCondition
)

// load search condition
func init() {
	jsonText, err := ioutil.ReadFile("../fixture/chair_condition.json")
	if err != nil {
		fmt.Printf("%v\n", err)
		os.Exit(1)
	}
	json.Unmarshal(jsonText, &chairSearchCondition)

	jsonText, err = ioutil.ReadFile("../fixture/estate_condition.json")
	if err != nil {
		fmt.Printf("%v\n", err)
		os.Exit(1)
	}
	json.Unmarshal(jsonText, &estateSearchCondition)
}

type InitializeResponse struct {
	Language string `json:"language"`
}

type Chair struct {
	ID             int64  `db:"id" json:"id"`
	Name           string `db:"name" json:"name"`
	Description    string `db:"description" json:"description"`
	Thumbnail      string `db:"thumbnail" json:"thumbnail"`
	Price          int64  `db:"price" json:"price"`
	Height         int64  `db:"height" json:"height"`
	Width          int64  `db:"width" json:"width"`
	Depth          int64  `db:"depth" json:"depth"`
	Color          string `db:"color" json:"color"`
	Features       string `db:"features" json:"features"`
	Kind           string `db:"kind" json:"kind"`
	Popularity     int64  `db:"popularity" json:"-"`
	PopularityDesc int64  `db:"popularity_desc" json:"-"`
	Stock          int64  `db:"stock" json:"-"`
}

type ChairSearchResponse struct {
	Count  int     `json:"count"`
	Chairs []*Chair `json:"chairs"`
}

type ChairListResponse struct {
	Chairs []*Chair `json:"chairs"`
}

//Estate 物件
type Estate struct {
	ID             int64   `db:"id" json:"id"`
	Thumbnail      string  `db:"thumbnail" json:"thumbnail"`
	Name           string  `db:"name" json:"name"`
	Description    string  `db:"description" json:"description"`
	Latitude       float64 `db:"latitude" json:"latitude"`
	Longitude      float64 `db:"longitude" json:"longitude"`
	Address        string  `db:"address" json:"address"`
	Rent           int64   `db:"rent" json:"rent"`
	DoorHeight     int64   `db:"door_height" json:"doorHeight"`
	DoorWidth      int64   `db:"door_width" json:"doorWidth"`
	Features       string  `db:"features" json:"features"`
	Popularity     int64   `db:"popularity" json:"-"`
	PopularityDesc int64   `db:"popularity_desc" json:"-"`
}

//EstateSearchResponse estate/searchへのレスポンスの形式
type EstateSearchResponse struct {
	Count   int64    `json:"count"`
	Estates []Estate `json:"estates"`
}

type EstateListResponse struct {
	Estates []Estate `json:"estates"`
}

type Coordinate struct {
	Latitude  float64 `json:"latitude"`
	Longitude float64 `json:"longitude"`
}

type Coordinates struct {
	Coordinates []Coordinate `json:"coordinates"`
}

type Range struct {
	ID  int64 `json:"id"`
	Min int64 `json:"min"`
	Max int64 `json:"max"`
}

type RangeCondition struct {
	Prefix string   `json:"prefix"`
	Suffix string   `json:"suffix"`
	Ranges []*Range `json:"ranges"`
}

type ListCondition struct {
	List []string `json:"list"`
}

type EstateSearchCondition struct {
	DoorWidth  RangeCondition `json:"doorWidth"`
	DoorHeight RangeCondition `json:"doorHeight"`
	Rent       RangeCondition `json:"rent"`
	Feature    ListCondition  `json:"feature"`
}

type ChairSearchCondition struct {
	Width   RangeCondition `json:"width"`
	Height  RangeCondition `json:"height"`
	Depth   RangeCondition `json:"depth"`
	Price   RangeCondition `json:"price"`
	Color   ListCondition  `json:"color"`
	Feature ListCondition  `json:"feature"`
	Kind    ListCondition  `json:"kind"`
}

type BoundingBox struct {
	// TopLeftCorner 緯度経度が共に最小値になるような点の情報を持っている
	TopLeftCorner Coordinate
	// BottomRightCorner 緯度経度が共に最大値になるような点の情報を持っている
	BottomRightCorner Coordinate
}

type MySQLConnectionEnv struct {
	Host     string
	Port     string
	User     string
	DBName   string
	Password string
}

type RecordMapper struct {
	Record []string

	offset int
	err    error
}

func (r *RecordMapper) next() (string, error) {
	if r.err != nil {
		return "", r.err
	}
	if r.offset >= len(r.Record) {
		r.err = fmt.Errorf("too many read")
		return "", r.err
	}
	s := r.Record[r.offset]
	r.offset++
	return s, nil
}

func (r *RecordMapper) NextInt() int {
	s, err := r.next()
	if err != nil {
		return 0
	}
	i, err := strconv.Atoi(s)
	if err != nil {
		r.err = err
		return 0
	}
	return i
}

func (r *RecordMapper) NextFloat() float64 {
	s, err := r.next()
	if err != nil {
		return 0
	}
	f, err := strconv.ParseFloat(s, 64)
	if err != nil {
		r.err = err
		return 0
	}
	return f
}

func (r *RecordMapper) NextString() string {
	s, err := r.next()
	if err != nil {
		return ""
	}
	return s
}

func (r *RecordMapper) Err() error {
	return r.err
}

func NewMySQLConnectionEnv() *MySQLConnectionEnv {
	return &MySQLConnectionEnv{
		Host:     getEnv("MYSQL_HOST", "127.0.0.1"),
		Port:     getEnv("MYSQL_PORT", "3306"),
		User:     getEnv("MYSQL_USER", "isucon"),
		DBName:   getEnv("MYSQL_DBNAME", "isuumo"),
		Password: getEnv("MYSQL_PASS", "isucon"),
	}
}

func getEnv(key, defaultValue string) string {
	val := os.Getenv(key)
	if val != "" {
		return val
	}
	return defaultValue
}

//ConnectDB isuumoデータベースに接続する
func (mc *MySQLConnectionEnv) ConnectDB() (*sqlx.DB, error) {
	dsn := fmt.Sprintf("%v:%v@tcp(%v:%v)/%v", mc.User, mc.Password, mc.Host, mc.Port, mc.DBName)
	return sqlx.Open("mysql", dsn)
}
