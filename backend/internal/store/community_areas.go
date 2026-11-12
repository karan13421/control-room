package store

import "github.com/jmoiron/sqlx"

type CommunityArea struct {
	ID   int    `db:"id" json:"id"`
	Name string `db:"name" json:"name"`
}

func ListCommunityAreas(db *sqlx.DB) ([]CommunityArea, error) {
	var areas []CommunityArea
	err := db.Select(&areas, `SELECT id, name FROM community_areas ORDER BY id`)
	return areas, err
}
