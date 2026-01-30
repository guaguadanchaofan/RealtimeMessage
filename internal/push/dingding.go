package push

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"
)

type DingTalk struct {
	Webhook   string
	Secret    string
	MsgType   string
	Title     string
	Timeout   time.Duration
	Template  string
}

type Response struct {
	ErrCode int    `json:"errcode"`
	ErrMsg  string `json:"errmsg"`
}

func (d *DingTalk) SendMarkdown(content string) error {
	payload := map[string]any{
		"msgtype": "markdown",
		"markdown": map[string]string{
			"title": d.Title,
			"text":  content,
		},
	}
	return d.send(payload)
}

func (d *DingTalk) SendText(content string) error {
	payload := map[string]any{
		"msgtype": "text",
		"text": map[string]string{
			"content": content,
		},
	}
	return d.send(payload)
}

func (d *DingTalk) send(payload map[string]any) error {
	ts := fmt.Sprintf("%d", time.Now().UnixMilli())
	sign := sign(ts, d.Secret)
	endpoint := fmt.Sprintf("%s&timestamp=%s&sign=%s", d.Webhook, ts, sign)
	buf, _ := json.Marshal(payload)
	client := &http.Client{Timeout: d.Timeout}
	resp, err := client.Post(endpoint, "application/json", bytes.NewReader(buf))
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	var r Response
	_ = json.NewDecoder(resp.Body).Decode(&r)
	if r.ErrCode != 0 {
		return fmt.Errorf("dingding error %d: %s", r.ErrCode, r.ErrMsg)
	}
	return nil
}

func sign(timestamp, secret string) string {
	stringToSign := fmt.Sprintf("%s\n%s", timestamp, secret)
	h := hmac.New(sha256.New, []byte(secret))
	_, _ = h.Write([]byte(stringToSign))
	encoded := base64.StdEncoding.EncodeToString(h.Sum(nil))
	return url.QueryEscape(encoded)
}

func RenderTemplate(tpl string, values map[string]string) string {
	res := tpl
	for k, v := range values {
		res = strings.ReplaceAll(res, "${"+k+"}", v)
	}
	return res
}
