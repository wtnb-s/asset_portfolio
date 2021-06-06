package main

import (
	"encoding/json"
	"fmt"
	"math"
	"os"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/guregu/dynamo"
)

type AssetUnit struct {
	AssetCode string
	Date      string
	Unit      int
}

type AssetUnitReq struct {
	AssetCode string `json:"AssetCode"`
	Date      string `json:"Date"`
	Amount    int    `json:"Amount"`
}

type AssetDaily struct {
	AssetCode string
	Date      string
	Price     int
}

func main() {
	lambda.Start(handler)
}

// メインハンドラー
func handler(request events.APIGatewayProxyRequest) (events.APIGatewayProxyResponse, error) {
	var (
		assetUnitData interface{}
		err           error
	)

	// リクエストがPOSTかGETで実行する処理を分岐する
	switch request.HTTPMethod {
	case "POST":
		// リクエストボディ取得
		reqBody := request.Body
		jsonBytes := ([]byte)(reqBody)
		assetUnitReq := new(AssetUnitReq)
		if err := json.Unmarshal(jsonBytes, assetUnitReq); err != nil {
			return events.APIGatewayProxyResponse{}, err
		}
		assetUnitData, err = postHandler(assetUnitReq)
	case "GET":
		// パス・クエリパラメータ取得
		assetCode := request.PathParameters["assetCode"]
		date := request.QueryStringParameters["date"]
		assetUnitData, err = getHandler(assetCode, date)
	}
	if err != nil {
		return events.APIGatewayProxyResponse{}, err
	}

	jsonBytes, _ := json.Marshal(assetUnitData)
	return events.APIGatewayProxyResponse{
		Headers: map[string]string{
			"Access-Control-Allow-Origin":      os.Getenv("ALLOW_ORIGIN"),
			"Access-Control-Allow-Headers":     "origin,Accept,Authorization,Content-Type",
			"Access-Control-Allow-Credentials": "true",
			"Content-Type":                     "application/json",
		},
		Body:       string(jsonBytes),
		StatusCode: 200,
	}, nil
}

// データ登録
func getHandler(assetCode string, date string) ([]AssetUnit, error) {
	var assetUnit []AssetUnit
	// Dynamodb接続
	table := connectDynamodb("asset_unit")
	// 資産データ取得
	if assetCode == "" {
		return nil, nil
	}
	filter := table.Scan().Filter("'AssetCode' = ?", assetCode)
	if date != "" {
		filter = filter.Filter("'Date' = ?", date)
	}
	err := filter.All(&assetUnit)
	return assetUnit, err
}

// データ登録
func postHandler(assetUnitReq *AssetUnitReq) (AssetUnit, error) {
	fmt.Println(assetUnitReq)
	assetCode := assetUnitReq.AssetCode
	date := assetUnitReq.Date
	amount := assetUnitReq.Amount

	// price からもamountを計算出来るようにする
	price, _ := getPrice(assetCode, date)
	fmt.Println(price)
	unit := math.Round(float64(amount) / float64(price) * 10000)
	fmt.Println(unit)

	assetAmount := AssetUnit{AssetCode: assetCode, Date: date, Unit: int(unit)}
	// Dynamodb接続
	table := connectDynamodb("asset_unit")
	// 資産データ登録
	err := table.Put(assetAmount).Run()

	return assetAmount, err
}

func getPrice(assetCode string, date string) (int, error) {
	var assetDailyData AssetDaily
	// Dynamodb接続
	table := connectDynamodb("asset_daily")
	// 資産価値データ取得
	if assetCode == "" && date == "" {
		return 0, nil
	}
	err := table.Get("AssetCode", assetCode).Range("Date", dynamo.Equal, date).One(&assetDailyData)
	price := assetDailyData.Price

	return price, err
}

// Dynamodb接続設定
func connectDynamodb(table string) dynamo.Table {
	// Endpoint設定(Local Dynamodb接続用)
	endpoint := os.Getenv("DYNAMODB_ENDPOINT")
	// Dynamodb接続設定
	session := session.Must(session.NewSession())
	config := aws.NewConfig().WithRegion("ap-northeast-1")
	if len(endpoint) > 0 {
		config = config.WithEndpoint(endpoint)
	}
	db := dynamo.New(session, config)
	return db.Table(table)
}
