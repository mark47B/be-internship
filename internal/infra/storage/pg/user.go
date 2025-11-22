package pg

import (
	"database/sql"

	"github.com/mark47B/be-internship/internal/domain/repository"
)

type UserStorage struct {
	db *sql.DB
}

func NewUserStorage(db *sql.DB) repository.UserRepository {
	return &UserStorage{db: db}
}
