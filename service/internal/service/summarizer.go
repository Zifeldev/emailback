package service

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"
)

type Client struct {
	token           string
	sumModels       map[string]string // language -> model
	clsModel        string
	httpClient      *http.Client
	hfBase          string
	defaultSumModel string
	mu              sync.RWMutex
	base            string
	http            *http.Client
}

func NewAIClient(token string, sumModels map[string]string, clsModel string, timeout time.Duration) *Client {
	if len(sumModels) == 0 {
		sumModels = map[string]string{
			"en": "sshleifer/distilbart-cnn-12-6",
			"ru": "IlyaGusev/mbart_ru_sum_gazeta",
			"de": "ml6team/mt5-small-german-finetune-mlsum",
		}
	}

	defaultModel := sumModels["en"]
	if defaultModel == "" {
		defaultModel = "sshleifer/distilbart-cnn-12-6"
	}

	return &Client{
		token:           token,
		sumModels:       sumModels,
		clsModel:        clsModel,
		defaultSumModel: defaultModel,
		httpClient:      &http.Client{Timeout: timeout},
		http:            &http.Client{Timeout: timeout},
		hfBase:          "https://router.huggingface.co/hf-inference/models",
		base:            "https://router.huggingface.co/hf-inference/models",
	}
}

func (c *Client) Summarize(ctx context.Context, text string, language string) (string, error) {
	model := c.sumModels[language]
	if model == "" {
		model = c.defaultSumModel
	}

	req := map[string]any{
		"inputs":     text,
		"parameters": map[string]any{"min_length": 20, "max_length": 120},
	}
	var resp []struct {
		SummaryText string `json:"summary_text"`
	}
	if err := c.post(ctx, model, req, &resp); err != nil {
		return "", err
	}
	if len(resp) == 0 {
		return "", fmt.Errorf("empty summarization response")
	}
	return resp[0].SummaryText, nil
}

type zeroShot struct {
	Labels []string  `json:"labels"`
	Scores []float64 `json:"scores"`
}

type zeroShotItem struct {
	Label string  `json:"label"`
	Score float64 `json:"score"`
}

func (c *Client) DetectPriority(ctx context.Context, text string) (label string, score float64, err error) {
	req := map[string]any{
		"inputs": text,
		"parameters": map[string]any{
			"candidate_labels": []string{"low", "medium", "high"},
			"multi_label":      false,
		},
	}
	var raw json.RawMessage
	if err := c.post(ctx, c.clsModel, req, &raw); err != nil {
		return "", 0, err
	}
	var items []zeroShotItem
	if json.Unmarshal(raw, &items) == nil && len(items) > 0 {
		best := items[0]
		for _, it := range items[1:] {
			if it.Score > best.Score {
				best = it
			}
		}
		return best.Label, best.Score, nil
	}
	var one zeroShot
	if json.Unmarshal(raw, &one) == nil && len(one.Labels) > 0 && len(one.Scores) > 0 {
		l, s := pick(one)
		return l, s, nil
	}
	sample := string(raw)
	if len(sample) > 200 {
		sample = sample[:200] + "..."
	}
	return "", 0, fmt.Errorf("unexpected zero-shot response format, raw sample: %s", sample)
}

func pick(z zeroShot) (string, float64) {
	i := 0
	for k := 1; k < len(z.Scores); k++ {
		if z.Scores[k] > z.Scores[i] {
			i = k
		}
	}
	return z.Labels[i], z.Scores[i]
}

func (c *Client) post(ctx context.Context, model string, payload any, out any) error {
	c.mu.RLock()
	base := c.base
	c.mu.RUnlock()
	u := fmt.Sprintf("%s/%s", base, model)
	b, _ := json.Marshal(payload)
	r, err := http.NewRequestWithContext(ctx, http.MethodPost, u, bytes.NewReader(b))
	if err != nil {
		return err
	}
	r.Header.Set("Authorization", "Bearer "+c.token)
	r.Header.Set("Content-Type", "application/json")
	res, err := c.http.Do(r)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	if res.StatusCode >= 300 {
		body, _ := io.ReadAll(res.Body)
		if res.StatusCode == http.StatusGone && strings.Contains(string(body), "no longer supported") {
			if strings.Contains(base, "api-inference.huggingface.co") {
				c.mu.Lock()
				if strings.Contains(c.base, "api-inference.huggingface.co") {
					c.base = "https://router.huggingface.co/hf-inference/models"
				}
				newBase := c.base
				c.mu.Unlock()
				retryReq, _ := http.NewRequestWithContext(ctx, http.MethodPost, fmt.Sprintf("%s/%s", newBase, model), bytes.NewReader(b))
				retryReq.Header.Set("Authorization", "Bearer "+c.token)
				retryReq.Header.Set("Content-Type", "application/json")
				if retryRes, retryErr := c.http.Do(retryReq); retryErr == nil {
					defer retryRes.Body.Close()
					if retryRes.StatusCode < 300 {
						return json.NewDecoder(retryRes.Body).Decode(out)
					}
					retryBody, _ := io.ReadAll(retryRes.Body)
					return fmt.Errorf("hf error: %s: %s", retryRes.Status, string(retryBody))
				} else {
					return fmt.Errorf("hf error retry failed: %v; original 410 body: %s", retryErr, string(body))
				}
			}
		}
		return fmt.Errorf("hf error: %s: %s", res.Status, string(body))
	}
	return json.NewDecoder(res.Body).Decode(out)
}

func (c *Client) Base() string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.base
}

func (c *Client) SumModel() string {
	return c.defaultSumModel
}

func (c *Client) ClsModel() string {
	return c.clsModel
}

func (c *Client) SumModels() map[string]string {
	return c.sumModels
}

func (c *Client) GetModelForLanguage(language string) string {
	c.mu.RLock()
	defer c.mu.RUnlock()

	model := c.sumModels[language]
	if model == "" {
		return c.defaultSumModel
	}
	return model
}
