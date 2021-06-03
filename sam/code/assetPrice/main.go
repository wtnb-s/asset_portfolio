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
	"time"

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
		err            error
	)

	// パス・クエリパラメータ取得
	assetCode := request.PathParameters["assetCode"]
	fromDate := request.QueryStringParameters["fromDate"]
	toDate := request.QueryStringParameters["toDate"]

	// リクエストがPOSTかGETで実行する処理を分岐する
	switch request.HTTPMethod {
	case "POST":
		assetDailyData, err = postHandler(assetCode, fromDate, toDate)
	case "GET":
		assetDailyData, err = getHandler(assetCode, fromDate, toDate)
	}
	if err != nil {
		return events.APIGatewayProxyResponse{}, err
	}

	jsonBytes, _ := json.Marshal(assetDailyData)
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
func postHandler(assetCode string, fromDate string, toDate string) (AssetDaily, error) {
	var assetDailyData AssetDaily

	// 日付設定
	splitfromDate := strings.Split(fromDate, "-")
	splitToDate := strings.Split(toDate, "-")
	fromYear := splitfromDate[0]
	fromMonth := splitfromDate[1]
	fromDay := splitfromDate[2]
	toYear := splitToDate[0]
	toMonth := splitToDate[1]
	toDay := splitToDate[2]

	// Post用のパラメータ設定
	values := url.Values{}
	values.Set("in_term_from_yyyy", fromYear)
	values.Set("in_term_from_mm", fromMonth)
	values.Set("in_term_from_dd", fromDay)
	values.Set("in_term_to_yyyy", toYear)
	values.Set("in_term_to_mm", toMonth)
	values.Set("in_term_to_dd", toDay)
	values.Set("dispRows", "100")
	// Dynamodb接続
	table := connectDynamodb("asset_daily")
	// データ1年毎に取得するため、ループは３回まで
	for page := 0; page < 3; page++ {
		values.Set("page", strconv.Itoa(page))
		// 基準価格取得
		dateList, priceList, _ := getPriceList(assetCode, values)
		// 不要な接続を防ぐため、ループを抜ける
		if len(dateList) == 0 {
			break
		}
		for idx, date := range dateList {
			price, _ := strconv.Atoi(priceList[idx])
			assetDailyData = AssetDaily{AssetCode: assetCode, Date: date, Price: price}

			// 資産価値データ登録
			err := registerAssetDailyData(table, assetDailyData)
			if err != nil {
				return assetDailyData, err
			}
		}
		// 間隔を開けて取得する
		time.Sleep(time.Second * 2)
	}
	return assetDailyData, nil
}

// データ登録
func getHandler(assetCode string, fromDate string, toDate string) ([]AssetDaily, error) {
	// Dynamodb接続
	table := connectDynamodb("asset_daily")
	// 資産価値データ取得
	assetDailyData, err := getAssetDailyDataByAssetCodeAndDate(table, assetCode, fromDate, toDate)
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
func getAssetDailyDataByAssetCodeAndDate(table dynamo.Table, assetCode string, fromDate string, toDate string) ([]AssetDaily, error) {
	var assetDailyData []AssetDaily
	if assetCode == "" {
		return nil, nil
	}
	filter := table.Scan().Filter("'AssetCode' = ?", assetCode)
	if fromDate != "" && toDate != "" {
		filter = filter.Filter("'Date' > ?", fromDate)
		filter = filter.Filter("'Date' < ?", toDate)
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

// 基準価格日取得
func getPriceList(assetCode string, params url.Values) ([]string, []string, error) {
	var valueList []string
	var dateList []string

	client := http.Client{}
	url := "https://site0.sbisec.co.jp/marble/fund/history/standardprice.do?fund_sec_code=" + assetCode

	// リクエスト発行
	req, err := http.NewRequest("POST", url, strings.NewReader(params.Encode()))
	if err != nil {
		return nil, nil, err
	}
	// ヘッダー追加
	req.Header.Add("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Add("User-Agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_13_5)” \"AppleWebKit/537.36 (KHTML, like Gecko) Chrome")

	// リクエスト送信
	resp, err := client.Do(req)
	if err != nil {
		return nil, nil, err
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

	// 基準価格日取得
	htmlDate := htmlDoc.Find("div.alC")
	htmlDate.Each(func(index int, s *goquery.Selection) {
		date := strings.Replace(s.Text(), "/", "-", -1)
		dateList = append(dateList, date)
	})
	// 基準価格取得
	htmlPrice := htmlDoc.Find("div.alR")
	htmlPrice.Each(func(index int, s *goquery.Selection) {
		if index%3 == 0 {
			tmpPrice := strings.Replace(s.Text(), "円", "", -1)
			price := strings.Replace(tmpPrice, ",", "", -1)
			valueList = append(valueList, price)
		}
	})
	return dateList, valueList, nil
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
