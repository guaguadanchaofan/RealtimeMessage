package parser

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/mmcdole/gofeed"

	"realtime-message/internal/config"
	"realtime-message/internal/model"
)

func ParseJSON(source string, body []byte, cfg config.ParserConfig) ([]model.Message, error) {
	var data any
	if err := json.Unmarshal(body, &data); err != nil {
		return nil, err
	}
	var items []any
	if strings.ToLower(cfg.Mode) == "mapping" && cfg.Mapping.ListPath != "" {
		items = findByPath(data, cfg.Mapping.ListPath)
	} else {
		items = autoFindList(data)
	}
	if items == nil {
		return nil, fmt.Errorf("no list found in json")
	}
	msgs := make([]model.Message, 0, len(items))
	for _, item := range items {
		obj, ok := item.(map[string]any)
		if !ok {
			continue
		}
		m := model.Message{Source: source}
		if strings.ToLower(cfg.Mode) == "mapping" {
			m = applyMapping(obj, m, cfg.Mapping)
		} else {
			m = applyAuto(obj, m)
		}
		if m.Time.IsZero() {
			m.Time = time.Now()
		}
		if m.Title == "" && m.Content == "" {
			continue
		}
		msgs = append(msgs, m)
	}
	return msgs, nil
}

func ParseRSS(source string, body []byte) ([]model.Message, error) {
	fp := gofeed.NewParser()
	feed, err := fp.ParseString(string(body))
	if err != nil {
		return nil, err
	}
	msgs := make([]model.Message, 0, len(feed.Items))
	for _, item := range feed.Items {
		t := time.Now()
		if item.PublishedParsed != nil {
			t = *item.PublishedParsed
		}
		msgs = append(msgs, model.Message{
			ID:      item.GUID,
			Title:   item.Title,
			Content: item.Description,
			URL:     item.Link,
			Time:    t,
			Source:  source,
		})
	}
	return msgs, nil
}

func applyMapping(obj map[string]any, msg model.Message, mapping config.MappingConfig) model.Message {
	fields := mapping.Fields
	if fields == nil {
		fields = map[string]string{}
	}
	msg.Title = getString(obj, fields["title"])
	msg.Content = getString(obj, fields["content"])
	msg.URL = getString(obj, fields["url"])
	msg.ID = getString(obj, fields["id"])
	msg.Time = parseTime(getAny(obj, fields["time"]))
	return msg
}

func applyAuto(obj map[string]any, msg model.Message) model.Message {
	msg.Title = firstNonEmpty(
		getString(obj, "title"),
		getString(obj, "headline"),
		getString(obj, "subject"),
		getString(obj, "title.text"),
		getString(obj, "title.rendered"),
	)
	msg.Content = firstNonEmpty(
		getString(obj, "content"),
		getString(obj, "summary"),
		getString(obj, "body"),
		getString(obj, "brief"),
		getString(obj, "description"),
		getString(obj, "content.text"),
		getString(obj, "content.rendered"),
	)
	msg.URL = firstNonEmpty(
		getString(obj, "url"),
		getString(obj, "link"),
		getString(obj, "share_url"),
	)
	msg.ID = firstNonEmpty(
		getString(obj, "id"),
		getString(obj, "guid"),
		getString(obj, "news_id"),
	)
	msg.Time = parseTime(firstNonNil(
		getAny(obj, "time"),
		getAny(obj, "timestamp"),
		getAny(obj, "published_at"),
		getAny(obj, "publish_time"),
	))
	return msg
}

func autoFindList(data any) []any {
	paths := []string{
		"data.list",
		"data.items",
		"data.result",
		"list",
		"items",
		"result",
		"data",
	}
	for _, path := range paths {
		if list := findByPath(data, path); list != nil {
			return list
		}
	}
	if list := findLikelyList(data, 0); list != nil {
		return list
	}
	if arr, ok := data.([]any); ok {
		return arr
	}
	return nil
}

func findByPath(data any, path string) []any {
	parts := strings.Split(path, ".")
	cur := data
	for _, p := range parts {
		if p == "" {
			continue
		}
		m, ok := cur.(map[string]any)
		if !ok {
			return nil
		}
		cur = m[p]
	}
	if arr, ok := cur.([]any); ok {
		return arr
	}
	return nil
}

func findLikelyList(data any, depth int) []any {
	if depth > 5 {
		return nil
	}
	switch v := data.(type) {
	case []any:
		if len(v) == 0 {
			return nil
		}
		if looksLikeMessageArray(v) {
			return v
		}
		for _, item := range v {
			if list := findLikelyList(item, depth+1); list != nil {
				return list
			}
		}
	case map[string]any:
		for _, item := range v {
			if list := findLikelyList(item, depth+1); list != nil {
				return list
			}
		}
	}
	return nil
}

func looksLikeMessageArray(arr []any) bool {
	for _, item := range arr {
		obj, ok := item.(map[string]any)
		if !ok {
			continue
		}
		for _, k := range []string{"title", "headline", "subject", "content", "summary", "brief"} {
			if _, ok := obj[k]; ok {
				return true
			}
		}
		if content, ok := obj["content"].(map[string]any); ok {
			if _, ok := content["text"]; ok {
				return true
			}
		}
	}
	return false
}

func getAny(obj map[string]any, key string) any {
	if key == "" {
		return nil
	}
	if strings.Contains(key, ".") {
		return getAnyByPath(obj, key)
	}
	return obj[key]
}

func getString(obj map[string]any, key string) string {
	val := getAny(obj, key)
	switch v := val.(type) {
	case string:
		return strings.TrimSpace(v)
	case float64:
		return strconv.FormatFloat(v, 'f', -1, 64)
	case int64:
		return strconv.FormatInt(v, 10)
	case json.Number:
		return v.String()
	default:
		return ""
	}
}

func getAnyByPath(obj map[string]any, path string) any {
	parts := strings.Split(path, ".")
	var cur any = obj
	for _, p := range parts {
		if p == "" {
			continue
		}
		m, ok := cur.(map[string]any)
		if !ok {
			return nil
		}
		cur = m[p]
	}
	return cur
}

func parseTime(val any) time.Time {
	switch v := val.(type) {
	case time.Time:
		return v
	case json.Number:
		if i, err := v.Int64(); err == nil {
			return fromUnix(i)
		}
	case float64:
		return fromUnix(int64(v))
	case int64:
		return fromUnix(v)
	case int:
		return fromUnix(int64(v))
	case string:
		return parseTimeString(v)
	default:
		return time.Time{}
	}
	return time.Time{}
}

func fromUnix(v int64) time.Time {
	if v > 1e12 {
		return time.UnixMilli(v)
	}
	return time.Unix(v, 0)
}

func parseTimeString(s string) time.Time {
	s = strings.TrimSpace(s)
	if s == "" {
		return time.Time{}
	}
	for _, layout := range []string{
		time.RFC3339,
		"2006-01-02 15:04:05",
		"2006-01-02 15:04",
		"2006-01-02",
	} {
		if t, err := time.ParseInLocation(layout, s, time.Local); err == nil {
			return t
		}
	}
	if i, err := strconv.ParseInt(s, 10, 64); err == nil {
		return fromUnix(i)
	}
	return time.Time{}
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if strings.TrimSpace(v) != "" {
			return v
		}
	}
	return ""
}

func firstNonNil(values ...any) any {
	for _, v := range values {
		if v != nil {
			return v
		}
	}
	return nil
}
