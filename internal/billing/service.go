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

// CreateSubscription creates a new subscription for a user
func (s *BillingService) CreateSubscription(ctx context.Context, user *models.User, planID string) error {
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

	// Create subscription
	subParams := &stripe.SubscriptionParams{
		Customer: &cus.ID,
		Items: []*stripe.SubscriptionItemsParams{
			{
				Price: &plan.PriceID,
			},
		},
	}

	sub, err := subscription.New(subParams)
	if err != nil {
		return fmt.Errorf("failed to create subscription: %w", err)
	}

	// Update user plan and credits
	user.Plan = planID
	user.Credits = plan.Credits

	// Update in DynamoDB
	err = s.updateUser(ctx, user)
	if err != nil {
		return fmt.Errorf("failed to update user: %w", err)
	}

	return nil
}

// DeductCredits removes credits from a user's account
func (s *BillingService) DeductCredits(ctx context.Context, userID string, amount int) error {
	user, err := s.getUser(ctx, userID)
	if err != nil {
		return fmt.Errorf("failed to get user: %w", err)
	}

	if user.Credits < amount {
		return ErrInsufficientCredits
	}

	user.Credits -= amount
	err = s.updateUser(ctx, user)
	if err != nil {
		return fmt.Errorf("failed to update credits: %w", err)
	}

	return nil
}

// AddCredits adds credits to a user's account
func (s *BillingService) AddCredits(ctx context.Context, userID string, amount int) error {
	user, err := s.getUser(ctx, userID)
	if err != nil {
		return fmt.Errorf("failed to get user: %w", err)
	}

	user.Credits += amount
	err = s.updateUser(ctx, user)
	if err != nil {
		return fmt.Errorf("failed to add credits: %w", err)
	}

	return nil
}

// GetUserCredits returns the number of credits a user has
func (s *BillingService) GetUserCredits(ctx context.Context, userID string) (int, error) {
	user, err := s.getUser(ctx, userID)
	if err != nil {
		return 0, fmt.Errorf("failed to get user credits: %w", err)
	}

	return user.Credits, nil
}

// Internal helper functions for DynamoDB operations
func (s *BillingService) getUser(ctx context.Context, userID string) (*models.User, error) {
	// TODO: Implement DynamoDB get user
	return &models.User{
		ID:        userID,
		Credits:   100,
		Plan:      "pro",
		CreatedAt: time.Now(),
	}, nil
}

func (s *BillingService) updateUser(ctx context.Context, user *models.User) error {
	// TODO: Implement DynamoDB update user
	return nil
}