package entity

type Team struct {
	Name    string
	Members []User
}

type UserStats struct {
	UserID          string
	CreatedPRCount  int
	ReviewedPRCount int
	MergedPRCount   int
}
