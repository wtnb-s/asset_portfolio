package main

import (
	"code/models"
	"encoding/json"
	"errors"
	"os"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
)

func main() {
	lambda.Start(handler)
}

// メインハンドラー
func handler(request events.APIGatewayProxyRequest) (events.APIGatewayProxyResponse, error) {
	// 変数初期化
	var assetDailyData []models.AssetDaily
	var err error

	// リクエストがPOSTかGETで実行する処理を分岐する
	switch request.HTTPMethod {
	case "POST":
		// リクエストボディ取得
		reqBody := request.Body
		jsonBytes := ([]byte)(reqBody)
		assetPriceReq := new(models.AssetPriceReq)
		if err = json.Unmarshal(jsonBytes, assetPriceReq); err != nil {
			return events.APIGatewayProxyResponse{}, err
		}
		// 資産タイプ（株 or 投資信託）
		assetType := assetPriceReq.AssetType
		// 資産コード
		assetCode := assetPriceReq.AssetCode
		if assetType == "stock" {
			// 対象地域(ex: US, JP...)
			region := assetPriceReq.Region
			// 取得対象期間(1d, 1mo, 1y)
			getRange := assetPriceReq.GetRange
			// 株価の時系列データを保存（Yahoo Finance APIから取得）
			err = models.SavePriceStock(region, assetCode, getRange)
		} else if assetType == "investmentTrust" {
			// 取得開始期間(yyyy-mm-dd)
			fromDate := assetPriceReq.FromDate
			// 取得終了期間(yyyy-mm-dd)
			toDate := assetPriceReq.ToDate
			// 投資信託の基準価格時系列データを保存
			err = models.SavePriceInvestmentTrust(assetCode, fromDate, toDate)
		} else {
			err = errors.New("no entered asset type")
		}
	case "GET":
		// パス・クエリパラメータ取得
		assetCode := request.PathParameters["assetCode"]
		fromDate := request.QueryStringParameters["fromDate"]
		toDate := request.QueryStringParameters["toDate"]
		assetDailyData, err = models.GetAssetPriceByAssetCodeAndDate(assetCode, fromDate, toDate)
	}
	if err != nil {
		return events.APIGatewayProxyResponse{}, err
	}

	jsonBytes, _ := json.Marshal(assetDailyData)
	return events.APIGatewayProxyResponse{
		Headers: map[string]string{
			"Access-Control-Allow-Origin":      os.Getenv("ALLOW_ORIGIN"),
			"Access-Control-Allow-Headers":     "X-Requested-With, Origin, X-Csrftoken, Content-Type, Accept",
			"Access-Control-Allow-Credentials": "true",
			"Content-Type":                     "application/json",
		},
		Body:       string(jsonBytes),
		StatusCode: 200,
	}, nil
}
