package models

type AssetMaster struct {
	AssetCode  string
	CategoryId string
	Name       string
	Type       int
}

type AssetMasterReq struct {
	AssetCode  string `json:"AssetCode"`
	CategoryId string `json:"CategoryId"`
	Name       string `json:"Name"`
	Type       int    `json:"Type"`
}

/*
 * 指定した資産コードまたはカテゴリーIDを元に資産マスタデータを取得
 */
func GetAssetMasterByAssetCodeAndCategoryId(assetCode string, categoryId string) ([]AssetMaster, error) {
	var assetMasterData []AssetMaster
	// Dynamodb接続
	table := connectDynamodb("asset_master")

	filter := table.Get("AssetCode", assetCode)
	if categoryId != "" {
		filter = filter.Filter("'CategoryId' = ?", categoryId)
	}
	err := filter.All(&assetMasterData)

	return assetMasterData, err
}

// 指定したコードの資産名取得
func GetAssetName(assetCode string) (string, error) {
	var assetMasterData []AssetMaster
	// Dynamodb接続
	table := connectDynamodb("asset_master")
	err := table.Get("AssetCode", assetCode).All(&assetMasterData)
	name := assetMasterData[0].Name

	return name, err
}

/*
 * 資産マスターデータを保存
 */
func SaveAssetMaster(assetMasterReq *AssetMasterReq) error {
	// Dynamodb接続
	table := connectDynamodb("asset_master")

	assetMasterData := AssetMaster{AssetCode: assetMasterReq.AssetCode, CategoryId: assetMasterReq.CategoryId,
		Type: assetMasterReq.Type, Name: assetMasterReq.Name}
	err := table.Put(assetMasterData).Run()
	if err != nil {
		return err
	}
	return nil
}
