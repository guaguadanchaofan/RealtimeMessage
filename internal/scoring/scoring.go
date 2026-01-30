package scoring

import (
	"strings"
	"time"

	"realtime-message/internal/config"
	"realtime-message/internal/model"
)

type Engine struct {
	Topics   []config.TopicConfig
	Triggers config.TriggerConfig
	Scoring  config.ScoringConfig
}

func (e *Engine) Score(msg model.Message) model.ScoredMessage {
	text := strings.ToLower(msg.Title + " " + msg.Content)
	score := 0
	reasons := []string{}

	score += msgBase(msg)
	if msgBase(msg) != 0 {
		reasons = append(reasons, "base")
	}

	for _, t := range e.Topics {
		if hitAny(text, t.Keywords) {
			score += t.Weight
			reasons = append(reasons, t.Name)
		}
	}
	if hitAny(text, e.Triggers.Strong.Keywords) {
		score += e.Triggers.Strong.Weight
		reasons = append(reasons, "strong")
	}

	if e.Scoring.MarketHours.Enabled {
		if inMarketHours(msg.Time) {
			score += e.Scoring.MarketHours.InSessionBonus
			reasons = append(reasons, "in_session")
		} else {
			score -= e.Scoring.MarketHours.OffSessionPenalty
			reasons = append(reasons, "off_session")
		}
	}

	return model.ScoredMessage{Message: msg, Score: score, Reasons: reasons}
}

func msgBase(msg model.Message) int {
	return msgBaseScore[msg.Source]
}

var msgBaseScore = map[string]int{}

func SetBaseScores(scores map[string]int) {
	msgBaseScore = scores
}

func hitAny(text string, keywords []string) bool {
	for _, k := range keywords {
		if k == "" {
			continue
		}
		if strings.Contains(text, strings.ToLower(k)) {
			return true
		}
	}
	return false
}

func inMarketHours(t time.Time) bool {
	lt := t.In(time.Local)
	h := lt.Hour()
	m := lt.Minute()
	// Default A-share session: 09:30-11:30 and 13:00-15:00
	if (h == 9 && m >= 30) || (h > 9 && h < 11) || (h == 11 && m <= 30) {
		return true
	}
	if (h == 13) || (h == 14) || (h == 15 && m == 0) {
		return true
	}
	return false
}
