package billing

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"github.com/celebthumb-ai/internal/models"
	"github.com/stripe/stripe-go/v76"
	"github.com/stripe/stripe-go/v76/customer"
	"github.com/stripe/stripe-go/v76/subscription"
)

var (
	ErrInsufficientCredits = errors.New("insufficient credits")
	ErrInvalidPlan        = errors.New("invalid subscription plan")
)

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

type BillingConfig struct {
	DynamoClient *dynamodb.Client
	TableName    string
	StripeKey    string
}

type BillingService struct {
	dynamoClient *dynamodb.Client
	tableName    string
	stripeKey    string
}

func NewBillingService(config BillingConfig) *BillingService {
	// Initialize Stripe
	stripe.Key = config.StripeKey
	
	return &BillingService{
		dynamoClient: config.DynamoClient,
		tableName:    config.TableName,
		stripeKey:    config.StripeKey,
	}
}

func (s *BillingService) GetUserCredits(ctx context.Context, userID string) (int, error) {
	// Get user from DynamoDB
	resp, err := s.dynamoClient.GetItem(ctx, &dynamodb.GetItemInput{
		TableName: aws.String(s.tableName),
		Key: map[string]types.AttributeValue{
			"id": &types.AttributeValueMemberS{Value: userID},
		},
	})
	if err != nil {
		return 0, fmt.Errorf("failed to get user: %w", err)
	}

	// If user doesn't exist, return 0 credits
	if resp.Item == nil {
		return 0, nil
	}

	// Unmarshal user
	var user models.User
	if err := attributevalue.UnmarshalMap(resp.Item, &user); err != nil {
		return 0, fmt.Errorf("failed to unmarshal user: %w", err)
	}

	return user.Credits, nil
}

func (s *BillingService) DeductCredits(ctx context.Context, userID string, amount int) error {
	// Get current credits
	credits, err := s.GetUserCredits(ctx, userID)
	if err != nil {
		return err
	}

	// Check if user has enough credits
	if credits < amount {
		return fmt.Errorf("insufficient credits")
	}

	// Update credits in DynamoDB
	_, err = s.dynamoClient.UpdateItem(ctx, &dynamodb.UpdateItemInput{
		TableName: aws.String(s.tableName),
		Key: map[string]types.AttributeValue{
			"id": &types.AttributeValueMemberS{Value: userID},
		},
		UpdateExpression: aws.String("SET credits = credits - :amount"),
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":amount": &types.AttributeValueMemberN{Value: fmt.Sprintf("%d", amount)},
		},
		ConditionExpression: aws.String("credits >= :amount"),
	})
	if err != nil {
		return fmt.Errorf("failed to deduct credits: %w", err)
	}

	return nil
}

func (s *BillingService) AddCredits(ctx context.Context, userID string, amount int) error {
	// Update credits in DynamoDB
	_, err := s.dynamoClient.UpdateItem(ctx, &dynamodb.UpdateItemInput{
		TableName: aws.String(s.tableName),
		Key: map[string]types.AttributeValue{
			"id": &types.AttributeValueMemberS{Value: userID},
		},
		UpdateExpression: aws.String("SET credits = if_not_exists(credits, :zero) + :amount"),
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":amount": &types.AttributeValueMemberN{Value: fmt.Sprintf("%d", amount)},
			":zero":   &types.AttributeValueMemberN{Value: "0"},
		},
	})
	if err != nil {
		return fmt.Errorf("failed to add credits: %w", err)
	}

	return nil
}

func (s *BillingService) CreateSubscription(ctx context.Context, user *models.User, planID string) error {
	// Check if plan exists
	plan, ok := Plans[planID]
	if !ok {
		return ErrInvalidPlan
	}

	// Create or update Stripe customer
	params := &stripe.CustomerParams{
		Email: &user.Email,
		Metadata: map[string]string{
			"userId": user.ID,
		},
	}
	
	cus, err := customer.New(params)
	if err != nil {
		return fmt.Errorf("failed to create stripe customer: %w", err)
	}

	// Create subscription if plan has a price ID
	if plan.PriceID != "" {
		subParams := &stripe.SubscriptionParams{
			Customer: &cus.ID,
			Items: []*stripe.SubscriptionItemsParams{
				{
					Price: &plan.PriceID,
				},
			},
		}

		_, err := subscription.New(subParams)
		if err != nil {
			return fmt.Errorf("failed to create subscription: %w", err)
		}
	}

	// Update user in DynamoDB
	item, err := attributevalue.MarshalMap(models.User{
		ID:        user.ID,
		Email:     user.Email,
		Plan:      planID,
		Credits:   plan.Credits,
		CreatedAt: time.Now(),
	})
	if err != nil {
		return fmt.Errorf("failed to marshal user: %w", err)
	}

	_, err = s.dynamoClient.PutItem(ctx, &dynamodb.PutItemInput{
		TableName: aws.String(s.tableName),
		Item:      item,
	})
	if err != nil {
		return fmt.Errorf("failed to create subscription: %w", err)
	}

	// Update user object
	user.Plan = planID
	user.Credits = plan.Credits

	return nil
}