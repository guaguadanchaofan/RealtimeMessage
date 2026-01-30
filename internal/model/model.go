package model

import "time"

type Message struct {
	ID      string
	Title   string
	Content string
	URL     string
	Time    time.Time
	Source  string
}

type ScoredMessage struct {
	Message
	Score   int
	Reasons []string
}
