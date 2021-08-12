package models

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
	"github.com/saintfish/chardet"
	"golang.org/x/net/html/charset"
)

type AssetDaily struct {
	AssetCode string
	Date      string
	Price     int
}

type AssetPriceReq struct {
	AssetType string `json:"AssetType"`
	AssetCode string `json:"AssetCode"`
	FromDate  string `json:"FromDate"`
	ToDate    string `json:"ToDate"`
	Region    string `json:"Region"`
	GetRange  string `json:"GetRange"`
}

type YahooFinanceStockData struct {
	Chart struct {
		Result []struct {
			Meta struct {
				Currency             string  `json:"currency"`
				Symbol               string  `json:"symbol"`
				ExchangeName         string  `json:"exchangeName"`
				InstrumentType       string  `json:"instrumentType"`
				FirstTradeDate       int     `json:"firstTradeDate"`
				RegularMarketTime    int     `json:"regularMarketTime"`
				Gmtoffset            int     `json:"gmtoffset"`
				Timezone             string  `json:"timezone"`
				ExchangeTimezoneName string  `json:"exchangeTimezoneName"`
				RegularMarketPrice   float64 `json:"regularMarketPrice"`
				ChartPreviousClose   float64 `json:"chartPreviousClose"`
				PriceHint            int     `json:"priceHint"`
				CurrentTradingPeriod struct {
					Pre struct {
						Timezone  string `json:"timezone"`
						Start     int    `json:"start"`
						End       int    `json:"end"`
						Gmtoffset int    `json:"gmtoffset"`
					} `json:"pre"`
					Regular struct {
						Timezone  string `json:"timezone"`
						Start     int    `json:"start"`
						End       int    `json:"end"`
						Gmtoffset int    `json:"gmtoffset"`
					} `json:"regular"`
					Post struct {
						Timezone  string `json:"timezone"`
						Start     int    `json:"start"`
						End       int    `json:"end"`
						Gmtoffset int    `json:"gmtoffset"`
					} `json:"post"`
				} `json:"currentTradingPeriod"`
				DataGranularity string   `json:"dataGranularity"`
				Range           string   `json:"range"`
				ValidRanges     []string `json:"validRanges"`
			} `json:"meta"`
			Timestamp  []int `json:"timestamp"`
			Indicators struct {
				Quote []struct {
					Low    []float64 `json:"low"`
					Close  []float64 `json:"close"`
					Volume []float64 `json:"volume"`
					Open   []float64 `json:"open"`
					High   []float64 `json:"high"`
				} `json:"quote"`
				Adjclose []struct {
					Adjclose []float64 `json:"adjclose"`
				} `json:"adjclose"`
			} `json:"indicators"`
		} `json:"result"`
		Error error `json:"error"`
	} `json:"chart"`
}

/*
 * 指定した資産コードまたは日付を元に資産価格データを取得
 */
func GetAssetPriceByAssetCodeAndDate(assetCode string, fromDate string, toDate string) ([]AssetDaily, error) {
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

/*
 * 投資信託の基準価格時系列データを保存（SBIからスクレイピングで取得）
 */
func SavePriceInvestmentTrust(assetCode string, fromDate string, toDate string) error {
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
		dateList, priceList, _ := GetListPriceInvestmentTrust(assetCode, values)
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
func GetListPriceInvestmentTrust(assetCode string, params url.Values) ([]string, []string, error) {
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

/*
 * 株価の時系列データを保存（Yahoo Finance APIから取得）
 */
func SavePriceStock(region string, assetCode string, getRange string) error {
	var assetDailyData AssetDaily
	// Dynamodb接続
	table := connectDynamodb("asset_daily")
	// 基準価格取得
	dateList, priceList, err := GetListPriceStock(region, assetCode, getRange)
	if err != nil {
		return err
	}

	for idx, date := range dateList {
		price := priceList[idx]
		assetDailyData = AssetDaily{AssetCode: assetCode, Date: date, Price: price}

		// 資産価値データ登録
		err := table.Put(assetDailyData).Run()
		if err != nil {
			return err
		}
	}
	return nil
}

// Yahoo Finance APIから指定したシンボルの株価を取得
func GetListPriceStock(region string, assetCode string, getRange string) ([]string, []int, error) {
	url := "https://apidojo-yahoo-finance-v1.p.rapidapi.com/stock/v2/get-chart?interval=1d&symbol=" + assetCode + "&range=" + getRange + "&region=" + region
	req, _ := http.NewRequest("GET", url, nil)
	req.Header.Add("x-rapidapi-key", os.Getenv("RAPIDAPI_Key"))
	req.Header.Add("x-rapidapi-host", "apidojo-yahoo-finance-v1.p.rapidapi.com")
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, nil, err
	}

	defer res.Body.Close()
	body, _ := ioutil.ReadAll(res.Body)
	// JSONデコード
	var yahooFinanceStockData YahooFinanceStockData
	if err := json.Unmarshal(body, &yahooFinanceStockData); err != nil {
		return nil, nil, err
	}
	// APIエラー判定
	if err := yahooFinanceStockData.Chart.Error; err != nil {
		return nil, nil, err
	}
	// 日付、価格取得
	timestampList := yahooFinanceStockData.Chart.Result[0].Timestamp
	adjcloseList := yahooFinanceStockData.Chart.Result[0].Indicators.Adjclose[0].Adjclose
	const layout = "2006-01-02"
	var dateList []string
	var priceList []int
	for idx, timestamp := range timestampList {
		// Unixタイムスタンプデータをyyyy-mm-dd形式に変換
		timeFull := time.Unix(int64(timestamp), 0)
		dateList = append(dateList, timeFull.Format(layout))
		// float64をintに変換
		priceList = append(priceList, int(adjcloseList[idx]))
	}

	return dateList, priceList, nil
}
