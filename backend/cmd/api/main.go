package main

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"os"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/rekognition"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/sagemaker"
	"github.com/celebthumb-ai/internal/ai"
	"github.com/celebthumb-ai/internal/billing"
	"github.com/celebthumb-ai/internal/models"
	"github.com/celebthumb-ai/internal/storage"
)

type API struct {
	aiService      *ai.AIService
	storageService *storage.StorageService
	billingService *billing.BillingService
}

func handleRequest(ctx context.Context, request events.APIGatewayProxyRequest) (events.APIGatewayProxyResponse, error) {
	// Initialize AWS SDK clients
	cfg, err := config.LoadDefaultConfig(ctx)
	if err != nil {
		return errorResponse(http.StatusInternalServerError, "failed to load AWS config"), nil
	}

	// Initialize services
	api := &API{
		aiService: ai.NewAIService(ai.AIConfig{
			RekognitionClient: rekognition.NewFromConfig(cfg),
			SagemakerClient:  sagemaker.NewFromConfig(cfg),
		}),
		storageService: storage.NewStorageService(storage.StorageConfig{
			S3Client: s3.NewFromConfig(cfg),
			Bucket:   os.Getenv("THUMBNAIL_BUCKET"),
		}),
		billingService: billing.NewBillingService(billing.BillingConfig{
			DynamoClient: dynamodb.NewFromConfig(cfg),
			TableName:    os.Getenv("USERS_TABLE"),
			StripeKey:    os.Getenv("STRIPE_SECRET_KEY"),
		}),
	}

	// Route request
	switch {
	case request.HTTPMethod == "POST" && request.Path == "/thumbnails":
		return api.handleGenerateThumbnail(ctx, request)
	case request.HTTPMethod == "GET" && request.Path == "/thumbnails":
		return api.handleListThumbnails(ctx, request)
	case request.HTTPMethod == "GET" && request.Resource == "/thumbnails/{id}":
		return api.handleGetThumbnail(ctx, request)
	case request.HTTPMethod == "DELETE" && request.Resource == "/thumbnails/{id}":
		return api.handleDeleteThumbnail(ctx, request)
	case request.HTTPMethod == "POST" && request.Path == "/subscriptions":
		return api.handleCreateSubscription(ctx, request)
	case request.HTTPMethod == "GET" && request.Path == "/credits":
		return api.handleGetCredits(ctx, request)
	default:
		return errorResponse(http.StatusNotFound, "not found"), nil
	}
}

func (api *API) handleGenerateThumbnail(ctx context.Context, request events.APIGatewayProxyRequest) (events.APIGatewayProxyResponse, error) {
	var req models.ThumbnailRequest
	if err := json.Unmarshal([]byte(request.Body), &req); err != nil {
		return errorResponse(http.StatusBadRequest, "invalid request"), nil
	}

	// Check credits
	if err := api.billingService.DeductCredits(ctx, req.UserID, 1); err != nil {
		return errorResponse(http.StatusPaymentRequired, "insufficient credits"), nil
	}

	// Generate thumbnail
	thumbnail, err := api.aiService.GenerateThumbnail(ctx, ai.GenerationParams{
		VideoTitle:  req.VideoTitle,
		Description: req.Description,
		Style:       req.Style,
	})
	if err != nil {
		return errorResponse(http.StatusInternalServerError, "failed to generate thumbnail"), nil
	}

	// Save thumbnail
	// Note: In a real implementation, we would get the image data from the AI service
	dummyData := []byte("dummy image data")
	if err := api.storageService.SaveThumbnail(ctx, thumbnail, dummyData); err != nil {
		return errorResponse(http.StatusInternalServerError, "failed to save thumbnail"), nil
	}

	return jsonResponse(http.StatusCreated, thumbnail)
}

func (api *API) handleListThumbnails(ctx context.Context, request events.APIGatewayProxyRequest) (events.APIGatewayProxyResponse, error) {
	userID := request.QueryStringParameters["userId"]
	if userID == "" {
		return errorResponse(http.StatusBadRequest, "userId is required"), nil
	}

	thumbnails, err := api.storageService.ListUserThumbnails(ctx, userID)
	if err != nil {
		return errorResponse(http.StatusInternalServerError, "failed to list thumbnails"), nil
	}

	return jsonResponse(http.StatusOK, thumbnails)
}

func (api *API) handleGetThumbnail(ctx context.Context, request events.APIGatewayProxyRequest) (events.APIGatewayProxyResponse, error) {
	userID := request.QueryStringParameters["userId"]
	thumbnailID := request.PathParameters["id"]

	data, err := api.storageService.GetThumbnail(ctx, userID, thumbnailID)
	if err != nil {
		return errorResponse(http.StatusInternalServerError, "failed to get thumbnail"), nil
	}

	return events.APIGatewayProxyResponse{
		StatusCode: http.StatusOK,
		Headers: map[string]string{
			"Content-Type": "image/jpeg",
		},
		Body:            string(data),
		IsBase64Encoded: true,
	}, nil
}

func (api *API) handleDeleteThumbnail(ctx context.Context, request events.APIGatewayProxyRequest) (events.APIGatewayProxyResponse, error) {
	userID := request.QueryStringParameters["userId"]
	thumbnailID := request.PathParameters["id"]

	if err := api.storageService.DeleteThumbnail(ctx, userID, thumbnailID); err != nil {
		return errorResponse(http.StatusInternalServerError, "failed to delete thumbnail"), nil
	}

	return events.APIGatewayProxyResponse{
		StatusCode: http.StatusNoContent,
	}, nil
}

func (api *API) handleCreateSubscription(ctx context.Context, request events.APIGatewayProxyRequest) (events.APIGatewayProxyResponse, error) {
	var req struct {
		UserID  string `json:"userId"`
		PlanID  string `json:"planId"`
		Email   string `json:"email"`
	}
	if err := json.Unmarshal([]byte(request.Body), &req); err != nil {
		return errorResponse(http.StatusBadRequest, "invalid request"), nil
	}

	user := &models.User{
		ID:    req.UserID,
		Email: req.Email,
	}

	if err := api.billingService.CreateSubscription(ctx, user, req.PlanID); err != nil {
		return errorResponse(http.StatusInternalServerError, "failed to create subscription"), nil
	}

	return jsonResponse(http.StatusCreated, user)
}

func (api *API) handleRegister(ctx context.Context, request events.APIGatewayProxyRequest) (events.APIGatewayProxyResponse, error) {
	var req struct {
		Email    string `json:"email"`
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if err := json.Unmarshal([]byte(request.Body), &req); err != nil {
		return errorResponse(http.StatusBadRequest, "invalid request"), nil
	}

	user, err := api.authService.RegisterUser(ctx, req.Email, req.Username, req.Password)
	if err != nil {
		if err == auth.ErrUserExists {
			return errorResponse(http.StatusConflict, "user already exists"), nil
		}
		return errorResponse(http.StatusInternalServerError, "failed to register user"), nil
	}

	return jsonResponse(http.StatusCreated, user)
}

func (api *API) handleLogin(ctx context.Context, request events.APIGatewayProxyRequest) (events.APIGatewayProxyResponse, error) {
	var req struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}
	if err := json.Unmarshal([]byte(request.Body), &req); err != nil {
		return errorResponse(http.StatusBadRequest, "invalid request"), nil
	}

	token, err := api.authService.LoginUser(ctx, req.Email, req.Password)
	if err != nil {
		if err == auth.ErrInvalidCredentials {
			return errorResponse(http.StatusUnauthorized, "invalid credentials"), nil
		}
		return errorResponse(http.StatusInternalServerError, "failed to login"), nil
	}

	return jsonResponse(http.StatusOK, map[string]string{"token": token})
}

func (api *API) handleGetCredits(ctx context.Context, request events.APIGatewayProxyRequest) (events.APIGatewayProxyResponse, error) {
	// Authenticate user
	token, err := auth.ExtractTokenFromRequest(request)
	if err != nil {
		return errorResponse(http.StatusUnauthorized, "unauthorized"), nil
	}

	user, err := api.authService.VerifyToken(ctx, token)
	if err != nil {
		return errorResponse(http.StatusUnauthorized, "invalid token"), nil
	}

	credits, err := api.billingService.GetUserCredits(ctx, user.ID)
	if err != nil {
		return errorResponse(http.StatusInternalServerError, "failed to get credits"), nil
	}

	return jsonResponse(http.StatusOK, map[string]int{"credits": credits})
}

func jsonResponse(statusCode int, body interface{}) (events.APIGatewayProxyResponse, error) {
	jsonBody, err := json.Marshal(body)
	if err != nil {
		return errorResponse(http.StatusInternalServerError, "failed to marshal response"), nil
	}

	return events.APIGatewayProxyResponse{
		StatusCode: statusCode,
		Headers: map[string]string{
			"Content-Type": "application/json",
		},
		Body: string(jsonBody),
	}, nil
}

func errorResponse(statusCode int, message string) events.APIGatewayProxyResponse {
	return events.APIGatewayProxyResponse{
		StatusCode: statusCode,
		Headers: map[string]string{
			"Content-Type": "application/json",
		},
		Body: fmt.Sprintf(`{"error":"%s"}`, message),
	}
}

func main() {
	lambda.Start(handleRequest)
}