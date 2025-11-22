package entity

import "time"

type PullRequest struct {
	ID                string
	Name              string
	AuthorID          string
	Status            PRStatus
	AssignedReviewers []string
	CreatedAt         *time.Time
	MergedAt          *time.Time
}

type PRStatus string

const (
	PROpen   PRStatus = "OPEN"
	PRMerged PRStatus = "MERGED"
)

type PRStats struct {
	Total             int
	Open              int
	Merged            int
	AvgReviewers      float64
	AvgMergeTimeHours float64
}
