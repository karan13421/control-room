package service

import (
	"github.com/jmoiron/sqlx"

	"control_room/internal/store"
)

type CommunityAreaService struct {
	db *sqlx.DB
}

func NewCommunityAreaService(db *sqlx.DB) *CommunityAreaService {
	return &CommunityAreaService{db: db}
}

func (s *CommunityAreaService) List() ([]store.CommunityArea, error) {
	return store.ListCommunityAreas(s.db)
}
