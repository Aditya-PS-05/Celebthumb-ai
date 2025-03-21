package storage

import (
	"context"
	"fmt"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/celebthumb-ai/internal/models"
)

type StorageService struct {
	s3Client *s3.Client
	bucket   string
}

type StorageConfig struct {
	S3Client *s3.Client
	Bucket   string
}

func NewStorageService(config StorageConfig) *StorageService {
	return &StorageService{
		s3Client: config.S3Client,
		bucket:   config.Bucket,
	}
}

// SaveThumbnail stores the generated thumbnail in S3 and returns the URL
func (s *StorageService) SaveThumbnail(ctx context.Context, thumbnail *models.Thumbnail, data []byte) error {
	key := fmt.Sprintf("thumbnails/%s/%s.jpg", thumbnail.UserID, thumbnail.ID)

	// Upload to S3
	_, err := s.s3Client.PutObject(ctx, &s3.PutObjectInput{
		Bucket:      aws.String(s.bucket),
		Key:         aws.String(key),
		Body:        aws.ReadSeekCloser(data),
		ContentType: aws.String("image/jpeg"),
		Metadata: map[string]string{
			"userId":      thumbnail.UserID,
			"videoTitle":  thumbnail.VideoTitle,
			"style":       thumbnail.Style,
			"created":     thumbnail.CreatedAt.Format(time.RFC3339),
		},
	})
	if err != nil {
		return fmt.Errorf("failed to upload thumbnail: %w", err)
	}

	// Set the public URL
	thumbnail.URL = fmt.Sprintf("https://%s.s3.amazonaws.com/%s", s.bucket, key)
	return nil
}

// GetThumbnail retrieves a thumbnail from S3
func (s *StorageService) GetThumbnail(ctx context.Context, userID, thumbnailID string) ([]byte, error) {
	key := fmt.Sprintf("thumbnails/%s/%s.jpg", userID, thumbnailID)

	result, err := s.s3Client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to get thumbnail: %w", err)
	}
	defer result.Body.Close()

	// Read the entire body
	data := make([]byte, *result.ContentLength)
	_, err = result.Body.Read(data)
	if err != nil {
		return nil, fmt.Errorf("failed to read thumbnail data: %w", err)
	}

	return data, nil
}

// ListUserThumbnails gets all thumbnails for a user
func (s *StorageService) ListUserThumbnails(ctx context.Context, userID string) ([]*models.Thumbnail, error) {
	prefix := fmt.Sprintf("thumbnails/%s/", userID)

	result, err := s.s3Client.ListObjectsV2(ctx, &s3.ListObjectsV2Input{
		Bucket: aws.String(s.bucket),
		Prefix: aws.String(prefix),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to list thumbnails: %w", err)
	}

	thumbnails := make([]*models.Thumbnail, 0, len(result.Contents))
	for _, obj := range result.Contents {
		// Parse metadata from object
		thumbnail := &models.Thumbnail{
			ID:        *obj.Key,
			UserID:    userID,
			URL:       fmt.Sprintf("https://%s.s3.amazonaws.com/%s", s.bucket, *obj.Key),
			CreatedAt: *obj.LastModified,
		}
		thumbnails = append(thumbnails, thumbnail)
	}

	return thumbnails, nil
}

// DeleteThumbnail removes a thumbnail from storage
func (s *StorageService) DeleteThumbnail(ctx context.Context, userID, thumbnailID string) error {
	key := fmt.Sprintf("thumbnails/%s/%s.jpg", userID, thumbnailID)

	_, err := s.s3Client.DeleteObject(ctx, &s3.DeleteObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		return fmt.Errorf("failed to delete thumbnail: %w", err)
	}

	return nil
}