package main

import (
	"bytes"
	"encoding/json"
	"io/ioutil"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"

	"github.com/PuerkitoBio/goquery"
	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/guregu/dynamo"
	"github.com/saintfish/chardet"
	"golang.org/x/net/html/charset"
)

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
		assetDailyData interface{}
		assetCode      string
		date           string
		err            error
	)

	// パス・クエリパラメータ取得
	assetCode = request.PathParameters["assetCode"]
	date = request.QueryStringParameters["date"]

	// リクエストがPOSTかGETで実行する処理を分岐する
	switch request.HTTPMethod {
	case "POST":
		assetDailyData, err = postHandler(assetCode, date)
	case "GET":
		assetDailyData, err = getHandler(assetCode, date)
	}
	if err != nil {
		return events.APIGatewayProxyResponse{}, err
	}

	jsonBytes, _ := json.Marshal(assetDailyData)
	return events.APIGatewayProxyResponse{
		Body:       string(jsonBytes),
		StatusCode: 200,
	}, nil
}

// データ登録
func postHandler(assetCode string, date string) (AssetDaily, error) {
	var assetDailyData AssetDaily

	// 日付データを年・月・日に分解
	splitDate := strings.Split(date, "-")
	year := splitDate[0]
	month := splitDate[1]
	day := splitDate[2]

	// Post用のパラメータ設定
	values := url.Values{}
	values.Add("in_term_from_yyyy", year)
	values.Add("in_term_from_mm", month)
	values.Add("in_term_from_dd", day)
	values.Add("in_term_to_yyyy", year)
	values.Add("in_term_to_mm", month)
	values.Add("in_term_to_dd", day)

	// url設定
	_url := "https://site0.sbisec.co.jp/marble/fund/history/standardprice.do?fund_sec_code=" + assetCode
	client := http.Client{}

	// リクエスト発行
	req, err := http.NewRequest("POST", _url, strings.NewReader(values.Encode()))
	if err != nil {
		return assetDailyData, err
	}
	// ヘッダー追加
	req.Header.Add("Content-Type", "application/x-www-form-urlencoded")
	// リクエスト送信
	resp, err := client.Do(req)
	if err != nil {
		return assetDailyData, err
	}

	defer resp.Body.Close()
	body, _ := ioutil.ReadAll(resp.Body)

	// 文字コード変換
	det := chardet.NewTextDetector()
	detRslt, _ := det.DetectBest(body)
	bReader := bytes.NewReader(body)
	reader, _ := charset.NewReaderLabel(detRslt.Charset, bReader)

	// HTMLパース
	htmlDoc, _ := goquery.NewDocumentFromReader(reader)

	// 基準価格を抜き出し
	basePriceText := htmlDoc.Find(".tdM").First().Text()
	// 抜き出した基準額を数値に変換
	basePriceTmp := strings.Replace(basePriceText, "円", "", 1)
	basePriceTmp = strings.Replace(basePriceTmp, ",", "", 1)
	basePrice, _ := strconv.Atoi(basePriceTmp)

	assetDailyData = AssetDaily{AssetCode: assetCode, Date: date, Price: basePrice}
	// Dynamodb接続
	table := connectDynamodb("asset_daily")
	// 資産価値データ登録
	err = registerAssetDailyData(table, assetDailyData)

	return assetDailyData, err
}

// データ登録
func getHandler(assetCode string, date string) ([]AssetDaily, error) {
	// Dynamodb接続
	table := connectDynamodb("asset_daily")
	// 資産価値データ取得
	assetDailyData, err := getAssetDailyDataByAssetCodeAndDate(table, assetCode, date)
	return assetDailyData, err
}

// 資産価値データ登録
func registerAssetDailyData(table dynamo.Table, assetDailyData AssetDaily) error {
	err := table.Put(assetDailyData).Run()
	if err != nil {
		return err
	}
	return nil
}

// 資産価値データ取得
func getAssetDailyDataByAssetCodeAndDate(table dynamo.Table, assetCode string, date string) ([]AssetDaily, error) {
	var assetDailyData []AssetDaily
	if assetCode == "" {
		return nil, nil
	}
	filter := table.Scan().Filter("'AssetCode' = ?", assetCode)
	if date != "" {
		filter = filter.Filter("'Date' = ?", date)
	}
	err := filter.All(&assetDailyData)
	return assetDailyData, err
}

// 資産価値データ取得
func getAssetDailyDataByAssetCodeAndAcquisition(table dynamo.Table, assetCode string, acquisition int16) ([]AssetDaily, error) {
	var assetDailyData []AssetDaily
	if assetCode == "" {
		return nil, nil
	}
	filter := table.Scan().Filter("'AssetCode' = ?", assetCode)
	if acquisition != 0 {
		filter = filter.Filter("'Acquisition' > ?", 0)
	}
	err := filter.All(&assetDailyData)
	return assetDailyData, err
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
