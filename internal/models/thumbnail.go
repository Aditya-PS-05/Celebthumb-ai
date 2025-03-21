package models

import (
	"time"

	"github.com/google/uuid"
)

type ThumbnailRequest struct {
	UserID      string    `json:"userId"`
	VideoTitle  string    `json:"videoTitle"`
	Description string    `json:"description"`
	Style       string    `json:"style"`
	CreatedAt   time.Time `json:"createdAt"`
}

type Thumbnail struct {
	ID          string    `json:"id"`
	UserID      string    `json:"userId"`
	URL         string    `json:"url"`
	VideoTitle  string    `json:"videoTitle"`
	Description string    `json:"description"`
	Style       string    `json:"style"`
	CreatedAt   time.Time `json:"createdAt"`
}

func NewThumbnail(req ThumbnailRequest) *Thumbnail {
	return &Thumbnail{
		ID:          uuid.New().String(),
		UserID:      req.UserID,
		VideoTitle:  req.VideoTitle,
		Description: req.Description,
		Style:       req.Style,
		CreatedAt:   time.Now(),
	}
}

type User struct {
	ID        string    `json:"id"`
	Email     string    `json:"email"`
	Plan      string    `json:"plan"`
	Credits   int       `json:"credits"`
	CreatedAt time.Time `json:"createdAt"`
}