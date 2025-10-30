package lang

import (
	"strings"

	"github.com/pemistahl/lingua-go"
)

type Detector interface {
	Detect(text string) (code string, confidence float64, ok bool)
}

type linguaDetector struct {
	detector lingua.LanguageDetector
	langs    []lingua.Language
}

func NewDetector(langs ...lingua.Language) *linguaDetector {
	if len(langs) == 0 {
		langs = []lingua.Language{
			lingua.English,
			lingua.Russian,
			lingua.German,
		}
	}
	d := lingua.NewLanguageDetectorBuilder().
		FromLanguages(langs...).
		WithMinimumRelativeDistance(0.0).
		Build()
	return &linguaDetector{detector: d, langs: langs}
}

func (l *linguaDetector) Detect(text string) (string, float64, bool) {
	trimmed := strings.TrimSpace(text)
	if trimmed == "" {
		return "", 0, false
	}

	detected, ok := l.detector.DetectLanguageOf(trimmed)
	if !ok {
		return "", 0, false
	}


	confVals := l.detector.ComputeLanguageConfidenceValues(trimmed)
	confF := extractConfidence(confVals, detected)


	iso := strings.ToLower(detected.IsoCode639_1().String())
	if iso != "" && iso != "unknown" {
		return iso, confF, true
	}


	switch detected {
	case lingua.English:
		return "en", confF, true
	case lingua.Russian:
		return "ru", confF, true
	case lingua.German:
		return "de", confF, true
	default:
		return strings.ToLower(detected.String()), confF, true
	}
}

func extractConfidence(confAny any, lang lingua.Language) float64 {
	switch m := confAny.(type) {
	case map[lingua.Language]float64:
		if v, ok := m[lang]; ok {
			return v
		}
	case map[lingua.Language]lingua.ConfidenceValue:
		if v, ok := m[lang]; ok {
			type hasValue interface{ Value() float64 }
			type hasScore interface{ Score() float64 }

			if hv, ok := any(v).(hasValue); ok {
				return hv.Value()
			}
			if hs, ok := any(v).(hasScore); ok {
				return hs.Score()
			}
			if f, ok := any(v).(float64); ok {
				return f
			}
		}
	}
	return 0
}
