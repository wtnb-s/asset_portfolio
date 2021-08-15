package main

import (
	"code/models"
	"encoding/json"
	"math"
	"os"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
)

type AssetTransition struct {
	Date   string
	Value  int
	Profit int
}

func main() {
	lambda.Start(handler)
}

/*
 * メインハンドラー
 * @param request httpリクエスト
 * return httpレスポンス
 */
func handler(request events.APIGatewayProxyRequest) (events.APIGatewayProxyResponse, error) {
	// 変数初期化
	var assetBuyData []models.AssetBuy
	var err error

	// 全ての資産購入データを取得し、AssetCode毎にリストを格納し、詰め直す
	assetBuyData, err = models.GetAssetBuyByAssetCode("")
	assetBuyDataByAssetCode := make(map[string][]models.AssetBuy)
	for _, data := range assetBuyData {
		assetBuyDataByAssetCode[data.AssetCode] = append(assetBuyDataByAssetCode[data.AssetCode], data)
	}

	// 資産価値の遷移
	var totalPastAssetValue [100]int
	// 損益の遷移
	var totalPastAssetProfit [100]int
	// 日付リスト
	var dateList [100]string

	// 全資産合計の過去100日間の資産価値と損益データを算出
	for assetCode, dataList := range assetBuyDataByAssetCode {
		sumUnit := 0
		sumAmount := 0
		for _, data := range dataList {
			sumUnit = sumUnit + data.Unit
			sumAmount = sumAmount + data.Amount
		}

		// 指定した資産の0〜100日前までの価格を取得
		priceList, _ := models.GetAssetPriceByAssetCodeAndDate(assetCode, "", "")
		priceListPast100day := priceList[len(priceList)-101 : len(priceList)-1]

		for idx, data := range priceListPast100day {
			pastAssetValue := int(math.Round(float64(data.Price) * float64(sumUnit) / 10000))
			totalPastAssetValue[idx] = totalPastAssetValue[idx] + pastAssetValue
			totalPastAssetProfit[idx] = totalPastAssetProfit[idx] + pastAssetValue - sumAmount
			dateList[idx] = data.Date
		}
	}

	// 構造体のスライス形式になるように形式を変換
	var tranditionDataList []AssetTransition
	for idx, date := range dateList {
		tranditionData := AssetTransition{Date: date, Value: totalPastAssetValue[idx], Profit: totalPastAssetProfit[idx]}
		tranditionDataList = append(tranditionDataList, tranditionData)
	}

	if err != nil {
		return events.APIGatewayProxyResponse{}, err
	}

	jsonBytes, _ := json.Marshal(tranditionDataList)
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
