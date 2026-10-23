package nativeanimation

import (
	"encoding/json"
	"os"
	"testing"
)

func TestAdmitPerformanceMatchesOriginalTypeScript(t *testing.T) {
	t.Parallel()
	_, c, _ := loadReference(t)
	raw, err := os.ReadFile("testdata/animation-fixtures.json")
	if err != nil {
		t.Fatal(err)
	}
	var ref struct {
		Admission []struct {
			Now             float64
			Text            string
			Input, Expected *Performance
		}
	}
	if err = json.Unmarshal(raw, &ref); err != nil {
		t.Fatal(err)
	}
	engine := NewEngine(c)
	for _, row := range ref.Admission {
		got := engine.AdmitPerformance(row.Input, row.Text, row.Now)
		assertJSONNumbers(t, got, row.Expected)
	}
}
