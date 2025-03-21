package billing

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/celebthumb-ai/internal/models"
	"github.com/stripe/stripe-go/v76"
	"github.com/stripe/stripe-go/v76/customer"
	"github.com/stripe/stripe-go/v76/subscription"
)

var (
	ErrInsufficientCredits = errors.New("insufficient credits")
	ErrInvalidPlan        = errors.New("invalid subscription plan")
)

type BillingService struct {
	dynamoClient *dynamodb.Client
	tableName    string
}

type BillingConfig struct {
	DynamoClient *dynamodb.Client
	TableName    string
	StripeKey    string
}

func NewBillingService(config BillingConfig) *BillingService {
	stripe.Key = config.StripeKey
	return &BillingService{
		dynamoClient: config.DynamoClient,
		tableName:    config.TableName,
	}
}

type Plan struct {
	ID            string
	Name          string
	PriceID       string
	Credits       int
	PricePerMonth float64
	Features      []string
}

var Plans = map[string]Plan{
	"free": {
		ID:            "free",
		Name:          "Free Tier",
		Credits:       10,
		PricePerMonth: 0,
		Features: []string{
			"10 thumbnails per month",
			"Basic styles",
			"Standard quality",
		},
	},
	"pro": {
		ID:            "pro",
		Name:          "Pro",
		PriceID:       "price_pro",
		Credits:       100,
		PricePerMonth: 29.99,
		Features: []string{
			"100 thumbnails per month",
			"Advanced styles",
			"HD quality",
			"Priority processing",
		},
	},
	"enterprise": {
		ID:            "enterprise",
		Name:          "Enterprise",
		PriceID:       "price_enterprise",
		Credits:       1000,
		PricePerMonth: 199.99,
		Features: []string{
			"1000 thumbnails per month",
			"Custom styles",
			"4K quality",
			"Dedicated support",
			"API access",
		},
	},
}