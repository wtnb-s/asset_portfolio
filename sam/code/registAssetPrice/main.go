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
	"github.com/aws/aws-sdk-go/service/dynamodb"
	"github.com/aws/aws-sdk-go/service/dynamodb/dynamodbattribute"
	"github.com/saintfish/chardet"
	"golang.org/x/net/html/charset"
)

type AssetDailyRes struct {
	AssetCode string `json:"AssetCode"`
	Date      string `json:"Date"`
	Price     int    `json:"Price"`
}

type AssetDaily struct {
	AssetCode string
	Date      string
	Price     int
}

func handler(request events.APIGatewayProxyRequest) (events.APIGatewayProxyResponse, error) {
	// 環境変数設定
	endpoint := os.Getenv("DYNAMODB_ENDPOINT")

	// パス・クエリパラメータ取得
	assetCode := request.PathParameters["assetCode"]
	date := request.QueryStringParameters["date"]

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
		return events.APIGatewayProxyResponse{}, err
	}
	// ヘッダー追加
	req.Header.Add("Content-Type", "application/x-www-form-urlencoded")
	// リクエスト送信
	resp, err := client.Do(req)
	if err != nil {
		return events.APIGatewayProxyResponse{}, err
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

	assetDaily := AssetDaily{
		AssetCode: assetCode,
		Date:      date,
		Price:     basePrice,
	}
	// item を dynamodb attributeに変換
	av, err := dynamodbattribute.MarshalMap(assetDaily)
	if err != nil {
		return events.APIGatewayProxyResponse{}, err
	}

	// Dynamodb接続設定
	session := session.Must(session.NewSession())
	config := aws.NewConfig().WithRegion("ap-northeast-1")
	if len(endpoint) > 0 {
		config = config.WithEndpoint(endpoint)
	}
	db := dynamodb.New(session, config)
	_, err = db.PutItem(&dynamodb.PutItemInput{
		TableName: aws.String("asset_daily"),
		Item:      av,
	})
	if err != nil {
		return events.APIGatewayProxyResponse{}, err
	}
	assetDailyRes := AssetDailyRes{
		AssetCode: assetCode,
		Date:      date,
		Price:     basePrice,
	}
	jsonBytes, _ := json.Marshal(assetDailyRes)
	return events.APIGatewayProxyResponse{
		Body:       string(jsonBytes),
		StatusCode: 200,
	}, nil

}

func main() {
	lambda.Start(handler)
}
