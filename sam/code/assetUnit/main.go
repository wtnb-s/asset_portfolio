package main

import (
	"encoding/json"
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
	Amount    int
}
type AssetUnitReq struct {
	AssetCode string `json:"AssetCode"`
	Date      string `json:"Date"`
	Unit      int    `json:"Unit"`
	Amount    int    `json:"Amount"`
}

type AssetMaster struct {
	AssetCode  string
	CategoryId string
	Name       string
	Type       int
}
type AssetDaily struct {
	AssetCode string
	Date      string
	Price     int
}
type AssetValue struct {
	Date   string
	Price  int
	Profit int
}

func main() {
	lambda.Start(handler)
}

// メインハンドラー
func handler(request events.APIGatewayProxyRequest) (events.APIGatewayProxyResponse, error) {
	// 変数初期化
	var assetUnitData []AssetUnit
	var err error
	unitDataList := make(map[string]map[string]interface{})

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
		err = postHandler(assetUnitReq)
	case "GET":
		// パス・クエリパラメータ取得
		assetCode := request.PathParameters["assetCode"]
		date := request.QueryStringParameters["date"]
		assetUnitData, err = getHandler(assetCode, date)

		// AssetCode毎にリストを格納
		assetUnitDataByAssetCode := make(map[string][]AssetUnit)
		for _, data := range assetUnitData {
			assetUnitDataByAssetCode[data.AssetCode] = append(assetUnitDataByAssetCode[data.AssetCode], data)
		}
		// 保持している資産の株数と平均取得単価を算出
		for assetCode, dataList := range assetUnitDataByAssetCode {
			sumUnit := 0
			sumAmount := 0
			for _, data := range dataList {
				sumUnit = sumUnit + data.Unit
				sumAmount = sumAmount + data.Amount
			}
			// 資産名取得
			assetName, _ := getAssetName(assetCode)
			// 指定した資産の最新の価格を取得
			priceList, _ := getPriceLatest100DaysByAssetCode(assetCode)

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

			// 過去１００日間の資産価値と損益の推移
			var pastAssetValueDataList []AssetValue
			for _, data := range priceList {
				pastAssetValue := int(math.Round(float64(data.Price) * float64(sumUnit) / 10000))
				pastAssetValueData := AssetValue{Date: data.Date, Price: pastAssetValue, Profit: pastAssetValue - sumAmount}
				pastAssetValueDataList = append(pastAssetValueDataList, pastAssetValueData)
			}
			unitData["AssetValueList"] = pastAssetValueDataList

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

// データ取得
func getHandler(assetCode string, date string) ([]AssetUnit, error) {
	var assetUnit []AssetUnit
	// Dynamodb接続
	table := connectDynamodb("asset_unit")
	filter := table.Scan()
	// 取得条件設定
	if assetCode != "" {
		filter = filter.Filter("'AssetCode' = ?", assetCode)
	}
	if date != "" {
		filter = filter.Filter("'Date' = ?", date)
	}
	err := filter.All(&assetUnit)
	return assetUnit, err
}

// データ登録
func postHandler(assetUnitReq *AssetUnitReq) error {
	assetCode := assetUnitReq.AssetCode
	date := assetUnitReq.Date
	amount := float64(assetUnitReq.Amount)
	unit := float64(assetUnitReq.Unit)

	// 対象日の基準価格を取得
	price, _ := getPriceByAssetCodeAndData(assetCode, date)
	// 金額を引数に口数を計算する
	if amount != 0 {
		unit = math.Round(float64(amount) / float64(price) * 10000)
	}
	// 口数を引数に金額を計算する
	if unit != 0 {
		amount = math.Round(float64(price) * float64(unit) / 10000)
	}

	assetAmount := AssetUnit{AssetCode: assetCode, Date: date, Unit: int(unit), Amount: int(amount)}
	// Dynamodb接続
	table := connectDynamodb("asset_unit")
	// 資産データ登録
	err := table.Put(assetAmount).Run()

	return err
}

// 指定した資産コードと日付の価格取得
func getPriceByAssetCodeAndData(assetCode string, date string) (int, error) {
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

// 指定した資産コードの最新の価格取得
func getPriceLatest100DaysByAssetCode(assetCode string) ([]AssetDaily, error) {
	var assetDailyData []AssetDaily
	// Dynamodb接続
	table := connectDynamodb("asset_daily")
	// 資産価値データ取得
	if assetCode == "" {
		return assetDailyData, nil
	}
	err := table.Get("AssetCode", assetCode).All(&assetDailyData)
	priceList := assetDailyData[len(assetDailyData)-101 : len(assetDailyData)-1]

	return priceList, err
}

// 指定したコードの資産名取得
func getAssetName(assetCode string) (string, error) {
	var assetMasterData []AssetMaster
	// Dynamodb接続
	table := connectDynamodb("asset_master")
	err := table.Get("AssetCode", assetCode).All(&assetMasterData)
	name := assetMasterData[0].Name

	return name, err
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
