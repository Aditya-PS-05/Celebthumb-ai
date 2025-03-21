package storage

import (
	"context"
	"fmt"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/celebthumb-ai/internal/models"
)

type StorageConfig struct {
	S3Client *s3.Client
	Bucket   string
}

type StorageService struct {
	s3Client *s3.Client
	bucket   string
}

func NewStorageService(config StorageConfig) *StorageService {
	return &StorageService{
		s3Client: config.S3Client,
		bucket:   config.Bucket,
	}
}

func (s *StorageService) SaveThumbnail(ctx context.Context, thumbnail *models.Thumbnail, data []byte) error {
	// Generate S3 key for the thumbnail
	key := fmt.Sprintf("thumbnails/%s/%s.jpg", thumbnail.UserID, thumbnail.ID)

	// Upload to S3
	_, err := s.s3Client.PutObject(ctx, &s3.PutObjectInput{
		Bucket:      aws.String(s.bucket),
		Key:         aws.String(key),
		Body:        aws.NewReadSeekCloser(data),
		ContentType: aws.String("image/jpeg"),
	})
	if err != nil {
		return fmt.Errorf("failed to upload thumbnail: %w", err)
	}

	// Set the URL
	thumbnail.URL = fmt.Sprintf("https://%s.s3.amazonaws.com/%s", s.bucket, key)

	return nil
}

func (s *StorageService) GetThumbnail(ctx context.Context, userID, thumbnailID string) ([]byte, error) {
	// Generate S3 key for the thumbnail
	key := fmt.Sprintf("thumbnails/%s/%s.jpg", userID, thumbnailID)

	// Get from S3
	resp, err := s.s3Client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to get thumbnail: %w", err)
	}
	defer resp.Body.Close()

	// Read the data
	data := make([]byte, resp.ContentLength)
	_, err = resp.Body.Read(data)
	if err != nil {
		return nil, fmt.Errorf("failed to read thumbnail data: %w", err)
	}

	return data, nil
}

func (s *StorageService) DeleteThumbnail(ctx context.Context, userID, thumbnailID string) error {
	// Generate S3 key for the thumbnail
	key := fmt.Sprintf("thumbnails/%s/%s.jpg", userID, thumbnailID)

	// Delete from S3
	_, err := s.s3Client.DeleteObject(ctx, &s3.DeleteObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		return fmt.Errorf("failed to delete thumbnail: %w", err)
	}

	return nil
}

func (s *StorageService) ListUserThumbnails(ctx context.Context, userID string) ([]*models.Thumbnail, error) {
	// Generate prefix for the user's thumbnails
	prefix := fmt.Sprintf("thumbnails/%s/", userID)

	// List objects in S3
	resp, err := s.s3Client.ListObjectsV2(ctx, &s3.ListObjectsV2Input{
		Bucket: aws.String(s.bucket),
		Prefix: aws.String(prefix),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to list thumbnails: %w", err)
	}

	// Convert to thumbnails
	thumbnails := make([]*models.Thumbnail, 0, len(resp.Contents))
	for _, obj := range resp.Contents {
		// Extract thumbnail ID from key
		key := *obj.Key
		id := key[len(prefix) : len(key)-4] // Remove prefix and .jpg extension

		// Create thumbnail
		thumbnail := &models.Thumbnail{
			ID:        id,
			UserID:    userID,
			URL:       fmt.Sprintf("https://%s.s3.amazonaws.com/%s", s.bucket, key),
			CreatedAt: *obj.LastModified,
		}

		thumbnails = append(thumbnails, thumbnail)
	}

	return thumbnails, nil
}