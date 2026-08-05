package model

type Archive struct {
	Month    string    `json:"month"`
	Count    int       `json:"count"`
	Articles []Article `json:"articles"`
}
