package models

import "math"

type AssetBuy struct {
	AssetCode string
	Date      string
	Unit      int
	Amount    int
}
type AssetBuyReq struct {
	AssetCode string `json:"AssetCode"`
	Date      string `json:"Date"`
	Unit      int    `json:"Unit"`
	Amount    int    `json:"Amount"`
}

/*
 * 指定した資産コードまたはカテゴリーIDを元に資産マスタデータを取得
 */
func GetAssetBuyByAssetCode(assetCode string) ([]AssetBuy, error) {
	var assetBuy []AssetBuy
	// Dynamodb接続
	table := connectDynamodb("asset_unit")
	filter := table.Scan()
	// 取得条件設定
	if assetCode != "" {
		filter = filter.Filter("'AssetCode' = ?", assetCode)
	}

	err := filter.All(&assetBuy)
	return assetBuy, err
}

/*
 * 購入資産データを保存
 */
func SaveAssetBuy(assetBuyReq *AssetBuyReq) error {
	assetCode := assetBuyReq.AssetCode
	date := assetBuyReq.Date
	amount := float64(assetBuyReq.Amount)
	unit := float64(assetBuyReq.Unit)

	// 対象日の基準価格を取得
	priceList, _ := GetAssetPriceByAssetCodeAndDate(assetCode, date, date)
	price := priceList[0].Price
	// 金額を引数に口数を計算する
	if amount != 0 {
		unit = math.Round(float64(amount) / float64(price) * 10000)
	}
	// 口数を引数に金額を計算する
	if unit != 0 {
		amount = math.Round(float64(price) * float64(unit) / 10000)
	}

	assetAmount := AssetBuy{AssetCode: assetCode, Date: date, Unit: int(unit), Amount: int(amount)}
	// Dynamodb接続
	table := connectDynamodb("asset_unit")
	// 資産データ登録
	err := table.Put(assetAmount).Run()

	return err
}
