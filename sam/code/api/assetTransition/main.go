package main

import (
	"code/models"
	"encoding/json"
	"math"
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
	var assetBuyData []models.AssetBuy
	var err error
	tranditionData := make(map[string]interface{})

	// 全ての資産購入データを取得し、AssetCode毎にリストを格納し、詰め直す
	assetBuyData, err = models.GetAssetBuyByAssetCode("")
	assetBuyDataByAssetCode := make(map[string][]models.AssetBuy)
	for _, data := range assetBuyData {
		assetBuyDataByAssetCode[data.AssetCode] = append(assetBuyDataByAssetCode[data.AssetCode], data)
	}

	// 資産価値の遷移
	var totalPstAssetValue [100]int
	// 損益の遷移
	var totalPastAssetProfit [100]int
	// 日付リスト
	var dateList [100]string
	for assetCode, dataList := range assetBuyDataByAssetCode {
		sumUnit := 0
		sumAmount := 0
		for _, data := range dataList {
			sumUnit = sumUnit + data.Unit
			sumAmount = sumAmount + data.Amount
		}

		// 指定した資産コードの資産の0〜100日前までの価格を取得
		priceList, _ := models.GetAssetPriceByAssetCodeAndDate(assetCode, "", "")
		priceListPast100day := priceList[len(priceList)-101 : len(priceList)-1]
		// 全資産合計の過去100日間の資産価値と損益データを取得
		for idx, data := range priceListPast100day {
			pastAssetValue := int(math.Round(float64(data.Price) * float64(sumUnit) / 10000))
			totalPstAssetValue[idx] = totalPstAssetValue[idx] + pastAssetValue
			totalPastAssetProfit[idx] = totalPastAssetProfit[idx] + pastAssetValue - sumAmount
			dateList[idx] = data.Date
		}
	}
	tranditionData["date"] = dateList
	tranditionData["value"] = totalPstAssetValue
	tranditionData["profit"] = totalPastAssetProfit

	if err != nil {
		return events.APIGatewayProxyResponse{}, err
	}

	jsonBytes, _ := json.Marshal(tranditionData)
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
