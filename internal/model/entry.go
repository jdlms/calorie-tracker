package model

// Entry is a single calorie tracking record.
type Entry struct {
	ID          string `json:"id"`
	Timestamp   int64  `json:"timestamp"`
	Description string `json:"description"`
	Kcal        int    `json:"kcal"`
	Protein     int    `json:"protein"`
	Fat         int    `json:"fat"`
	Carbs       int    `json:"carbs"`
}
