package main

import (
	"encoding/json"
	"os"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/guregu/dynamo"
)

type AssetMaster struct {
	AssetCode  string
	CategoryId string
	Name       string
	Type       int
}

type AssetMasterReq struct {
	AssetCode  string `json:"AssetCode"`
	CategoryId string `json:"CategoryId"`
	Name       string `json:"Name"`
	Type       int    `json:"Type"`
}

func main() {
	lambda.Start(handler)
}

// メインハンドラー
func handler(request events.APIGatewayProxyRequest) (events.APIGatewayProxyResponse, error) {
	var (
		assetMasterData interface{}
		assetCode       string
		categoryId      string
		err             error
	)

	// リクエストがPOSTかGETで実行する処理を分岐する
	switch request.HTTPMethod {
	case "POST":
		// リクエストボディ取得
		reqBody := request.Body
		jsonBytes := ([]byte)(reqBody)
		assetMasterReq := new(AssetMasterReq)
		if err := json.Unmarshal(jsonBytes, assetMasterReq); err != nil {
			return events.APIGatewayProxyResponse{}, err
		}
		assetMasterData, err = postHandler(assetMasterReq)
	case "GET":
		// パス・クエリパラメータ取得
		assetCode = request.PathParameters["assetCode"]
		categoryId = request.QueryStringParameters["date"]
		assetMasterData, err = getHandler(assetCode, categoryId)
	}
	if err != nil {
		return events.APIGatewayProxyResponse{}, err
	}

	jsonBytes, _ := json.Marshal(assetMasterData)
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
func getHandler(assetCode string, categoryId string) ([]AssetMaster, error) {
	// Dynamodb接続
	table := connectDynamodb("asset_master")
	// 資産価値データ取得
	assetDailyData, err := getAssetMasterDataByAssetCodeAndCategoryId(table, assetCode, categoryId)
	return assetDailyData, err
}

// データ登録
func postHandler(assetMasterReq *AssetMasterReq) (AssetMaster, error) {
	assetCode := assetMasterReq.AssetCode
	assetCategoryId := assetMasterReq.CategoryId
	assetType := assetMasterReq.Type
	assetName := assetMasterReq.Name

	assetMasterData := AssetMaster{AssetCode: assetCode, CategoryId: assetCategoryId, Type: assetType, Name: assetName}
	// Dynamodb接続
	table := connectDynamodb("asset_master")
	// 資産データ登録
	err := registerAssetMasterData(table, assetMasterData)

	return assetMasterData, err
}

// 資産データ登録
func registerAssetMasterData(table dynamo.Table, assetMasterData AssetMaster) error {
	err := table.Put(assetMasterData).Run()
	if err != nil {
		return err
	}
	return nil
}

// 資産データ取得
func getAssetMasterDataByAssetCodeAndCategoryId(table dynamo.Table, assetCode string, categoryId string) ([]AssetMaster, error) {
	var assetMasterData []AssetMaster
	if assetCode == "" {
		return nil, nil
	}
	filter := table.Scan().Filter("'AssetCode' = ?", assetCode)
	if categoryId != "" {
		filter = filter.Filter("'CategoryId' = ?", categoryId)
	}
	err := filter.All(&assetMasterData)
	return assetMasterData, err
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
