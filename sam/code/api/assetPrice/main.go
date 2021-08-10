package main

import (
	"code/models"
	"encoding/json"
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

	// パス・クエリパラメータ取得
	assetCode := request.PathParameters["assetCode"]
	fromDate := request.QueryStringParameters["fromDate"]
	toDate := request.QueryStringParameters["toDate"]

	// リクエストがPOSTかGETで実行する処理を分岐する
	switch request.HTTPMethod {
	case "POST":
		err = models.SavePriceInvestmentTrust(assetCode, fromDate, toDate)
	case "GET":
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
