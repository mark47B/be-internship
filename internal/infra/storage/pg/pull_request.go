package pg

import (
	"database/sql"

	"github.com/mark47B/be-internship/internal/domain/repository"
)

type PullRequestStorage struct {
	db *sql.DB
}

func NewPullRequestStorage(db *sql.DB) repository.PullRequestRepository {
	return &PullRequestStorage{db: db}
}
