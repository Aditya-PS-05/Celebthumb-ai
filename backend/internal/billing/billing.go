package billing

import (
	"context"
	"fmt"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"github.com/celebthumb-ai/internal/models"
)

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
	// In a real implementation, this would create a subscription in Stripe
	// For now, we'll just update the user's plan and add credits

	// Determine credits based on plan
	var credits int
	switch planID {
	case "basic":
		credits = 50
	case "pro":
		credits = 200
	case "enterprise":
		credits = 1000
	default:
		return fmt.Errorf("invalid plan ID")
	}

	// Update user in DynamoDB
	item, err := attributevalue.MarshalMap(models.User{
		ID:        user.ID,
		Email:     user.Email,
		Plan:      planID,
		Credits:   credits,
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
	user.Credits = credits

	return nil
}