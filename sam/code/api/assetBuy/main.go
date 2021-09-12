package main

import (
	"code/config"
	"code/models"
	"encoding/json"
	"math"
	"os"
	"strconv"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
)

type UnitDataList struct {
	Detail   []UnitDataDetail
	Category [7]UnitDataCategory
}

type UnitDataDetail struct {
	AssetCode        string
	AssetName        string
	TotalUnit        int
	TotalBuyPrice    int
	PresentValue     int
	AvaregeUnitPrice int
}
type UnitDataCategory struct {
	AssetCode     string
	AssetName     string
	TotalBuyPrice int
	PresentValue  int
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
	var err error
	var unitDataList UnitDataList

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
		// 変数初期化
		var unitDataDetailList []UnitDataDetail
		var unitDataCategoryList [7]UnitDataCategory
		// カテゴリコードとカテゴリー名を設定する
		assetCategoryList := map[int]string{1: "日本株", 2: "先進国株", 3: "新興株", 4: "先進国債券", 5: "新興国債券", 6: "コモディティ", 7: "暗号資産"}
		for code, name := range assetCategoryList {
			unitDataCategoryList[code-1].AssetCode = strconv.Itoa(code)
			unitDataCategoryList[code-1].AssetName = name
		}

		// パス・クエリパラメータ取得
		assetCode := request.PathParameters["assetCode"]
		assetBuyData, _ := models.GetAssetBuyByAssetCode(assetCode)

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
			assetMaster, _ := models.GetAssetMasterByAssetCodeAndCategoryId(assetCode, "")
			assetName := assetMaster[0].Name
			assetCategoryId, _ := strconv.Atoi(assetMaster[0].CategoryId)

			// 投資信託であれば、基準価格=1万口に合わせて、算出する
			basePriceConstant := 1
			if assetMaster[0].Type == config.ASSET_TYPE_INVESTMENT_TRUST {
				basePriceConstant = 10000
			}

			// 指定した資産の直近価格を取得
			priceList, _ := models.GetAssetPriceByAssetCodeAndDate(assetCode, "", "")
			// 現在価値
			currentPrice := priceList[len(priceList)-1].Price
			presentValue := int(math.Round(float64(currentPrice) * float64(sumUnit) / float64(basePriceConstant)))

			unitDataDetail := UnitDataDetail{
				// 資産コード
				AssetCode: assetCode,
				// 資産名
				AssetName: assetName,
				// 保持株数
				TotalUnit: sumUnit,
				// 合計購入価格
				TotalBuyPrice: sumAmount,
				// 現在価値
				PresentValue: presentValue,
				// 平均購入単価
				AvaregeUnitPrice: basePriceConstant * sumAmount / sumUnit,
			}
			// 資産データをリストに追加
			unitDataDetailList = append(unitDataDetailList, unitDataDetail)

			// 資産タイプ毎にまとめる
			index := assetCategoryId - 1
			unitDataCategoryList[index].PresentValue = unitDataCategoryList[index].PresentValue + sumAmount
			unitDataCategoryList[index].TotalBuyPrice = unitDataCategoryList[index].TotalBuyPrice + presentValue
		}
		unitDataList = UnitDataList{Detail: unitDataDetailList, Category: unitDataCategoryList}
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
