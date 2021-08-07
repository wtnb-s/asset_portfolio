package models

import (
	"bytes"
	"io/ioutil"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
	"github.com/saintfish/chardet"
	"golang.org/x/net/html/charset"
)

type AssetDaily struct {
	AssetCode string
	Date      string
	Price     int
}

/*
 * 指定した資産コードまたは日付を元に資産価格データを取得
 */
func GetAssetPriceByAssetCodeOrDate(assetCode string, fromDate string, toDate string) ([]AssetDaily, error) {
	var assetDailyData []AssetDaily
	// Dynamodb接続
	table := connectDynamodb("asset_daily")

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

// 指定した資産コードの最新の価格取得
func GetPriceLatest100DaysByAssetCode(assetCode string) ([]AssetDaily, error) {
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

/*
 * 資産価格データを保存
 */
func SaveAssetPrice(assetCode string, fromDate string, toDate string) error {
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
		dateList, priceList, _ := GetListAssetPrice(assetCode, values)
		// 不要な接続を防ぐため、ループを抜ける
		if len(dateList) == 0 {
			break
		}
		for idx, date := range dateList {
			price, _ := strconv.Atoi(priceList[idx])
			assetDailyData = AssetDaily{AssetCode: assetCode, Date: date, Price: price}

			// 資産価値データ登録
			err := table.Put(assetDailyData).Run()
			if err != nil {
				return err
			}
		}
		// 間隔を開けて取得する
		time.Sleep(time.Second * 5)
	}
	return nil
}

// sbiのHPに接続し、基準価格をスクレイピングで取得
func GetListAssetPrice(assetCode string, params url.Values) ([]string, []string, error) {
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
