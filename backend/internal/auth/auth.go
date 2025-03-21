package auth

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/cognitoidentityprovider"
	"github.com/aws/aws-sdk-go-v2/service/cognitoidentityprovider/types"
	"github.com/celebthumb-ai/internal/models"
	"github.com/golang-jwt/jwt/v4"
	"github.com/lestrrat-go/jwx/jwk"
)

var (
	ErrInvalidToken      = errors.New("invalid token")
	ErrExpiredToken      = errors.New("token expired")
	ErrInvalidCredentials = errors.New("invalid credentials")
	ErrUserNotFound      = errors.New("user not found")
	ErrUserExists        = errors.New("user already exists")
)

type AuthConfig struct {
	CognitoClient *cognitoidentityprovider.Client
	UserPoolID    string
	ClientID      string
}

type AuthService struct {
	cognitoClient *cognitoidentityprovider.Client
	userPoolID    string
	clientID      string
	jwkSet        jwk.Set
}

func NewAuthService(config AuthConfig) *AuthService {
	userPoolID := config.UserPoolID
	if userPoolID == "" {
		userPoolID = os.Getenv("USER_POOL_ID")
	}

	clientID := config.ClientID
	if clientID == "" {
		clientID = os.Getenv("USER_POOL_CLIENT_ID")
	}

	// Fetch JWK Set for token validation
	jwksURL := fmt.Sprintf("https://cognito-idp.%s.amazonaws.com/%s/.well-known/jwks.json", 
		os.Getenv("AWS_REGION"), userPoolID)
	jwkSet, err := jwk.Fetch(context.Background(), jwksURL)
	if err != nil {
		// Log error but continue - we'll validate tokens without JWK if needed
		fmt.Printf("Error fetching JWK set: %v\n", err)
	}

	return &AuthService{
		cognitoClient: config.CognitoClient,
		userPoolID:    userPoolID,
		clientID:      clientID,
		jwkSet:        jwkSet,
	}
}

func (s *AuthService) RegisterUser(ctx context.Context, email, username, password string) (*models.User, error) {
	// Check if user already exists
	_, err := s.cognitoClient.AdminGetUser(ctx, &cognitoidentityprovider.AdminGetUserInput{
		UserPoolId: aws.String(s.userPoolID),
		Username:   aws.String(email),
	})
	if err == nil {
		return nil, ErrUserExists
	}

	// Create user in Cognito
	_, err = s.cognitoClient.SignUp(ctx, &cognitoidentityprovider.SignUpInput{
		ClientId: aws.String(s.clientID),
		Username: aws.String(email),
		Password: aws.String(password),
		UserAttributes: []types.AttributeType{
			{
				Name:  aws.String("email"),
				Value: aws.String(email),
			},
			{
				Name:  aws.String("preferred_username"),
				Value: aws.String(username),
			},
		},
	})
	if err != nil {
		return nil, fmt.Errorf("failed to register user: %w", err)
	}

	// Auto-confirm user for development purposes
	// In production, you would use email verification
	_, err = s.cognitoClient.AdminConfirmSignUp(ctx, &cognitoidentityprovider.AdminConfirmSignUpInput{
		UserPoolId: aws.String(s.userPoolID),
		Username:   aws.String(email),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to confirm user: %w", err)
	}

	// Create and return user model
	user := &models.User{
		ID:        email,
		Email:     email,
		Plan:      "free",
		Credits:   10, // Give new users some free credits
		CreatedAt: time.Now(),
	}

	return user, nil
}

func (s *AuthService) LoginUser(ctx context.Context, email, password string) (string, error) {
	// Authenticate user with Cognito
	resp, err := s.cognitoClient.InitiateAuth(ctx, &cognitoidentityprovider.InitiateAuthInput{
		AuthFlow: types.AuthFlowTypeUserPasswordAuth,
		ClientId: aws.String(s.clientID),
		AuthParameters: map[string]string{
			"USERNAME": email,
			"PASSWORD": password,
		},
	})
	if err != nil {
		return "", ErrInvalidCredentials
	}

	// Return the ID token
	return *resp.AuthenticationResult.IdToken, nil
}

func (s *AuthService) VerifyToken(ctx context.Context, token string) (*models.User, error) {
	// Parse and validate the JWT token
	parser := jwt.Parser{
		ValidMethods: []string{"RS256"},
	}

	parsedToken, err := parser.Parse(token, func(token *jwt.Token) (interface{}, error) {
		// Verify the token signing method
		if _, ok := token.Method.(*jwt.SigningMethodRSA); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}

		// Get the key ID from the token header
		kid, ok := token.Header["kid"].(string)
		if !ok {
			return nil, errors.New("kid header not found")
		}

		// Find the key in the JWK set
		if key, found := s.jwkSet.LookupKeyID(kid); found {
			var rawKey interface{}
			if err := key.Raw(&rawKey); err != nil {
				return nil, err
			}
			return rawKey, nil
		}

		return nil, errors.New("key not found")
	})

	if err != nil {
		return nil, ErrInvalidToken
	}

	if !parsedToken.Valid {
		return nil, ErrInvalidToken
	}

	// Extract claims
	claims, ok := parsedToken.Claims.(jwt.MapClaims)
	if !ok {
		return nil, ErrInvalidToken
	}

	// Verify token expiration
	if exp, ok := claims["exp"].(float64); ok {
		if time.Unix(int64(exp), 0).Before(time.Now()) {
			return nil, ErrExpiredToken
		}
	}

	// Extract user information
	email, _ := claims["email"].(string)
	sub, _ := claims["sub"].(string)

	// Create and return user model
	user := &models.User{
		ID:    sub,
		Email: email,
	}

	return user, nil
}

// ExtractTokenFromRequest extracts the JWT token from the Authorization header
func ExtractTokenFromRequest(request events.APIGatewayProxyRequest) (string, error) {
	authHeader := request.Headers["Authorization"]
	if authHeader == "" {
		return "", ErrInvalidToken
	}

	// Check if the header has the Bearer prefix
	if !strings.HasPrefix(authHeader, "Bearer ") {
		return "", ErrInvalidToken
	}

	// Extract the token
	token := strings.TrimPrefix(authHeader, "Bearer ")
	if token == "" {
		return "", ErrInvalidToken
	}

	return token, nil
}