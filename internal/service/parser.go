package service

import (
	"bytes"
	"context"
	"mime"
	"net/mail"
	"strings"
	"time"

	"github.com/Zifeldev/emailback/internal/lang"
	"github.com/Zifeldev/emailback/internal/repository"
	"github.com/google/uuid"
	"github.com/jhillyerd/enmime"
)


type Options struct {
	IncludeHTML     bool
	HTMLToTextLimit int
}

type Parser interface {
	Parse(ctx context.Context, rawEmail []byte) (*repository.EmailEntity, error)
}

type EnmimeParser struct {
	opts     Options
	detector lang.Detector
}

func NewEnmimeParser(opts Options, detector lang.Detector) *EnmimeParser {
	if opts.HTMLToTextLimit <= 0 {
		opts.HTMLToTextLimit = 1 << 20 // 1 MB
	}
	return &EnmimeParser{opts: opts, detector: detector}
}

func (p *EnmimeParser) Parse(_ context.Context, raw []byte) (*repository.EmailEntity, error) {
	env, err := enmime.ReadEnvelope(bytes.NewReader(raw))
	if err != nil {
		return nil, err
	}


	headers := make(map[string]string, 32)
	if env.Root != nil {
		for k, vals := range env.Root.Header {
			if len(vals) == 0 {
				continue
			}
			headers[strings.ToLower(k)] = strings.Join(vals, ", ")
		}
	}

	msgRaw := env.GetHeader("Message-ID")
	msgID := strings.Trim(msgRaw, " <>")
	if msgID == "" {
		msgID = uuid.NewString()
	}


	subjRaw := env.GetHeader("Subject")
	subject := subjRaw
	if subjRaw != "" {
		if dec, err := (&mime.WordDecoder{}).DecodeHeader(subjRaw); err == nil {
			subject = dec
		}
	}

	from := env.GetHeader("From")
	if from == "" {
		if addrs, err := env.AddressList("From"); err == nil && len(addrs) > 0 {
			from = addrs[0].Address
		}
	} else {
		if a, err := mail.ParseAddress(from); err == nil {
			from = a.Address
		}
	}

	var toList []string
	if vals := env.GetHeaderValues("To"); len(vals) > 0 {
		for _, v := range vals {
			if addrs, err := mail.ParseAddressList(v); err == nil {
				for _, a := range addrs {
					toList = append(toList, a.Address)
				}
			}
		}
	}

	var datePtr *time.Time
	if dv := env.GetHeader("Date"); dv != "" {
		if dt, err := mail.ParseDate(dv); err == nil {
			datePtr = &dt
		}
	}

	body := strings.TrimSpace(env.Text)
	if body == "" && env.HTML != "" {
		body = env.HTML
	}


	clean := lang.CleanText(body)


	var langCode string
	var langConf float64
	if p.detector != nil && strings.TrimSpace(clean) != "" {
		if code, conf, ok := p.detector.Detect(clean); ok {
			langCode = code
			langConf = conf
		} else {
			_ = code
			_ = conf
		}
	}


	var attachments []map[string]interface{}
	for _, a := range env.Attachments {
		meta := map[string]interface{}{
			"filename": a.FileName,
			"size":     len(a.Content),
			"ctype":    a.ContentType,
		}
		attachments = append(attachments, meta)
	}

	// Metrics
	metrics := map[string]interface{}{
		"char_count":   len([]rune(clean)),
		"word_count":   countWords(clean),
		"line_count":   lineCount(clean),
		"subject_len":  len([]rune(subject)),
		"headers_size": len(headers),
		"attachments":  len(attachments),
	}

	entity := &repository.EmailEntity{
		ID:         uuid.NewString(),
		MessageID:  msgID,
		From:       from,
		To:         toList,
		Subject:    subject,
		Date:       datePtr,
		Text:       clean,
		HTML:       pickHTML(env.HTML, p.opts.IncludeHTML),
		Language:   langCode,
		Confidence: langConf,
		Metrics:    metrics,
		Headers:    headers,
		CreatedAt:  time.Now().UTC(),
		RawSize:    len(raw),
	}

	return entity, nil
}

func pickHTML(html string, include bool) string {
	if include {
		return html
	}
	return ""
}

func countWords(s string) int {
	if s = strings.TrimSpace(s); s == "" {
		return 0
	}
	n := 0
	for _, w := range strings.Fields(s) {
		if w != "" {
			n++
		}
	}
	return n
}

func lineCount(s string) int {
	if s == "" {
		return 0
	}
	return strings.Count(s, "\n") + 1
}