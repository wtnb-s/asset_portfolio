package main

import (
	"encoding/json"
	"os"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/dynamodb"
	"github.com/aws/aws-sdk-go/service/dynamodb/dynamodbattribute"
)

func handler(request events.APIGatewayProxyRequest) (events.APIGatewayProxyResponse, error) {
	// 環境変数設定
	endpoint := os.Getenv("DYNAMODB_ENDPOINT")
	// パスパラメータ取得
	assetCode := request.QueryStringParameters["assetCode"]
	categoryId := request.QueryStringParameters["categoryId"]

	// Dynamodb接続設定
	session := session.Must(session.NewSession())
	config := aws.NewConfig().WithRegion("ap-northeast-1")
	if len(endpoint) > 0 {
		config = config.WithEndpoint(endpoint)
	}
	db := dynamodb.New(session, config)

	param, err := db.Query(&dynamodb.QueryInput{
		TableName: aws.String("asset_master"),
		ExpressionAttributeNames: map[string]*string{
			"#AssetCode":  aws.String("AssetCode"),
			"#CategoryId": aws.String("CategoryId"),
			"#Name":       aws.String("Name"),
			"#Type":       aws.String("Type"),
		},
		ExpressionAttributeValues: map[string]*dynamodb.AttributeValue{
			":assetCode": {
				S: aws.String(assetCode),
			},
			":categoryId": {
				S: aws.String(categoryId),
			},
		},

		KeyConditionExpression: aws.String("#AssetCode=:assetCode AND #CategoryId=:categoryId"),
		ProjectionExpression:   aws.String("#AssetCode, #CategoryId, #Name, #Type"),
	})
	if err != nil {
		return events.APIGatewayProxyResponse{}, err
	}

	assetMaster := make([]*AssetMasterRes, 0)
	if err := dynamodbattribute.UnmarshalListOfMaps(param.Items, &assetMaster); err != nil {
		return events.APIGatewayProxyResponse{}, err
	}
	jsonBytes, _ := json.Marshal(assetMaster)

	return events.APIGatewayProxyResponse{
		Body:       string(jsonBytes),
		StatusCode: 200,
	}, nil
}

func main() {
	lambda.Start(handler)
}

type AssetMasterRes struct {
	AssetCode  string `json:"AssetCode"`
	CategoryId string `json:"CategoryId"`
	Name       string `json:"Name"`
	Type       int    `json:"Type"`
}
