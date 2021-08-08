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
	unitDataList := make(map[string]map[string]interface{})

	// リクエストがPOSTかGETで実行する処理を分岐する
	switch request.HTTPMethod {
	case "POST":
		// リクエストボディ取得
		reqBody := request.Body
		jsonBytes := ([]byte)(reqBody)
		assetBuyReq := new(models.AssetBuyReq)
		if err := json.Unmarshal(jsonBytes, assetBuyReq); err != nil {
			return events.APIGatewayProxyResponse{}, err
		}
		err = models.SaveAssetBuy(assetBuyReq)
	case "GET":
		// パス・クエリパラメータ取得
		assetCode := request.PathParameters["assetCode"]
		assetBuyData, err = models.GetAssetBuyByAssetCode(assetCode)

		// AssetCode毎にリストを格納
		assetBuyDataByAssetCode := make(map[string][]models.AssetBuy)
		for _, data := range assetBuyData {
			assetBuyDataByAssetCode[data.AssetCode] = append(assetBuyDataByAssetCode[data.AssetCode], data)
		}
		// 保持している資産の株数と平均取得単価を算出
		for assetCode, dataList := range assetBuyDataByAssetCode {
			sumUnit := 0
			sumAmount := 0
			for _, data := range dataList {
				sumUnit = sumUnit + data.Unit
				sumAmount = sumAmount + data.Amount
			}
			// 資産名取得
			assetName, _ := models.GetAssetName(assetCode)
			// 指定した資産の最新の価格を取得
			priceList, _ := models.GetAssetPriceByAssetCodeAndDate(assetCode, "", "")

			unitData := make(map[string]interface{})
			// 資産名
			unitData["assetName"] = assetName
			// 保持株数
			unitData["sumUnit"] = sumUnit
			// 合計購入価格
			unitData["acquisitionPrice"] = sumAmount
			// 現在価値
			currentPrice := priceList[len(priceList)-1].Price
			unitData["presentValue"] = int(math.Round(float64(currentPrice) * float64(sumUnit) / 10000))
			// 平均購入単価
			unitData["avaregeUnitPrice"] = 10000 * sumAmount / sumUnit

			// 資産データをリストに追加
			unitDataList[assetCode] = unitData
		}
	}
	if err != nil {
		return events.APIGatewayProxyResponse{}, err
	}

	jsonBytes, _ := json.Marshal(unitDataList)
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
