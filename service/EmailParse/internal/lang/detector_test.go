package lang

import (
	"testing"

	"github.com/pemistahl/lingua-go"
)

func TestLinguaDetector(t *testing.T) {
	d := NewDetector(lingua.English, lingua.Russian, lingua.German)

	cases := []struct {
		text string
		want string
	}{
		{"Hello, how are you?", "en"},
		{"Привет, как дела?", "ru"},
		{"Guten Tag! Wie geht's?", "de"},
	}

	for _, c := range cases {
		got, conf, ok := d.Detect(c.text)
		if !ok || got != c.want {
			t.Fatalf("Detect(%q) got=%s conf=%.3f ok=%v; want=%s", c.text, got, conf, ok, c.want)
		}
	}
}
