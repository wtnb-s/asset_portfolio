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

/*
 * メインハンドラー
 * @param request httpリクエスト
 * return httpレスポンス
 */
func handler(request events.APIGatewayProxyRequest) (events.APIGatewayProxyResponse, error) {
	var assetMasterData []models.AssetMaster
	var assetCode string
	var categoryId string
	var err error

	// リクエストがPOSTかGETで実行する処理を分岐する
	switch request.HTTPMethod {
	case "POST":
		// リクエストボディ取得
		reqBody := request.Body
		jsonBytes := ([]byte)(reqBody)
		assetMasterReq := new(models.AssetMasterReq)
		if err := json.Unmarshal(jsonBytes, assetMasterReq); err != nil {
			return events.APIGatewayProxyResponse{}, err
		}
		err = models.SaveAssetMaster(assetMasterReq)
	case "GET":
		// パス・クエリパラメータ取得
		assetCode = request.QueryStringParameters["assetCode"]
		categoryId = request.QueryStringParameters["date"]
		assetMasterData, err = models.GetAssetMasterByAssetCodeAndCategoryId(assetCode, categoryId)
	}
	if err != nil {
		return events.APIGatewayProxyResponse{}, err
	}

	jsonBytes, _ := json.Marshal(assetMasterData)
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
