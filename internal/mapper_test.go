package internal

import (
	"crypto/sha256"
	"encoding/hex"
	"testing"
	"transbroker/internal/domain"
)

func hash(language, path, value string) string {
	sum := sha256.Sum256([]byte(language + ":" + path + ":" + value))
	return hex.EncodeToString(sum[:])
}

func TestMapJSONToDataList_FlatFields(t *testing.T) {
	raw := []byte(`{
		"language": "en",
		"data": {
			"title": "Hello",
			"date": "01.01.2026"
		}
	}`)

	result, err := MapJSONToDataList("req-1", raw)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.Language != "en" {
		t.Errorf("Language: got %q, want %q", result.Language, "en")
	}
	if result.RequestHash != "req-1" {
		t.Errorf("RequestHash: got %q, want %q", result.RequestHash, "req-1")
	}
	if len(result.Data) != 2 {
		t.Fatalf("Data len: got %d, want 2", len(result.Data))
	}

	cases := []struct {
		path  string
		value string
	}{
		{"title", "Hello"},
		{"date", "01.01.2026"},
	}
	for _, c := range cases {
		d, ok := result.Data[c.path]
		if !ok {
			t.Errorf("key %q not found in Data", c.path)
			continue
		}
		if d.Text != c.value {
			t.Errorf("Data[%q].Text: got %q, want %q", c.path, d.Text, c.value)
		}
		want := hash("en", c.path, c.value)
		if d.TextHash != want {
			t.Errorf("Data[%q].TextHash: got %q, want %q", c.path, d.TextHash, want)
		}
	}
}

func TestMapJSONToDataList_NestedFields(t *testing.T) {
	raw := []byte(`{
		"language": "de",
		"data": {
			"section": {
				"title": "Titel",
				"note": "Notiz"
			}
		}
	}`)

	result, err := MapJSONToDataList("req-2", raw)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(result.Data) != 2 {
		t.Fatalf("Data len: got %d, want 2", len(result.Data))
	}

	cases := []struct {
		path  string
		value string
	}{
		{"section.title", "Titel"},
		{"section.note", "Notiz"},
	}
	for _, c := range cases {
		d, ok := result.Data[c.path]
		if !ok {
			t.Errorf("key %q not found in Data", c.path)
			continue
		}
		if d.Text != c.value {
			t.Errorf("Data[%q].Text: got %q, want %q", c.path, d.Text, c.value)
		}
		want := hash("de", c.path, c.value)
		if d.TextHash != want {
			t.Errorf("Data[%q].TextHash: got %q, want %q", c.path, d.TextHash, want)
		}
	}
}

func TestMapJSONToDataList_DeepNesting(t *testing.T) {
	raw := []byte(`{
		"language": "fr",
		"data": {
			"a": {
				"b": {
					"c": "deep value"
				}
			}
		}
	}`)

	result, err := MapJSONToDataList("req-3", raw)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	d, ok := result.Data["a.b.c"]
	if !ok {
		t.Fatal("key \"a.b.c\" not found in Data")
	}
	if d.Text != "deep value" {
		t.Errorf("Text: got %q, want %q", d.Text, "deep value")
	}
	want := hash("fr", "a.b.c", "deep value")
	if d.TextHash != want {
		t.Errorf("TextHash: got %q, want %q", d.TextHash, want)
	}
}

func TestMapJSONToDataList_MissingLanguage(t *testing.T) {
	raw := []byte(`{"data": {"title": "Hello"}}`)

	_, err := MapJSONToDataList("req-4", raw)
	if err == nil {
		t.Fatal("expected error for missing language, got nil")
	}
}

func TestMapJSONToDataList_InvalidJSON(t *testing.T) {
	_, err := MapJSONToDataList("req-5", []byte(`not json`))
	if err == nil {
		t.Fatal("expected error for invalid JSON, got nil")
	}
}

func TestMapOutputToNested_Flat(t *testing.T) {
	dataList := domain.DataList{
		Language: "en",
		Data: map[string]domain.Data{
			"title": {TextHash: hash("en", "title", "Привет"), Text: "Привет"},
			"date":  {TextHash: hash("en", "date", "01.01.2026"), Text: "01.01.2026"},
		},
	}
	outputs := []domain.DataOutput{
		{TextHash: hash("en", "title", "Привет"), TranslatedText: "Hello"},
		{TextHash: hash("en", "date", "01.01.2026"), TranslatedText: "01.01.2026"},
	}

	nested := MapOutputToNested(dataList, outputs)

	if nested["title"] != "Hello" {
		t.Errorf("title: got %q, want %q", nested["title"], "Hello")
	}
	if nested["date"] != "01.01.2026" {
		t.Errorf("date: got %q, want %q", nested["date"], "01.01.2026")
	}
}

func TestMapOutputToNested_Nested(t *testing.T) {
	dataList := domain.DataList{
		Language: "en",
		Data: map[string]domain.Data{
			"phpDev.title": {TextHash: hash("en", "phpDev.title", "PHP разработчик"), Text: "PHP разработчик"},
			"phpDev.note":  {TextHash: hash("en", "phpDev.note", "Только"), Text: "Только"},
			"title":        {TextHash: hash("en", "title", "Варианты"), Text: "Варианты"},
		},
	}
	outputs := []domain.DataOutput{
		{TextHash: hash("en", "phpDev.title", "PHP разработчик"), TranslatedText: "PHP developer"},
		{TextHash: hash("en", "phpDev.note", "Только"), TranslatedText: "Only"},
		{TextHash: hash("en", "title", "Варианты"), TranslatedText: "Options"},
	}

	nested := MapOutputToNested(dataList, outputs)

	if nested["title"] != "Options" {
		t.Errorf("title: got %q, want %q", nested["title"], "Options")
	}
	phpDev, ok := nested["phpDev"].(map[string]interface{})
	if !ok {
		t.Fatal("phpDev is not a map")
	}
	if phpDev["title"] != "PHP developer" {
		t.Errorf("phpDev.title: got %q, want %q", phpDev["title"], "PHP developer")
	}
	if phpDev["note"] != "Only" {
		t.Errorf("phpDev.note: got %q, want %q", phpDev["note"], "Only")
	}
}

func TestMapOutputToNested_FallbackToOriginal(t *testing.T) {
	dataList := domain.DataList{
		Language: "en",
		Data: map[string]domain.Data{
			"title": {TextHash: hash("en", "title", "Привет"), Text: "Привет"},
		},
	}
	// No matching output — should fall back to original Text
	nested := MapOutputToNested(dataList, []domain.DataOutput{})

	if nested["title"] != "Привет" {
		t.Errorf("title: got %q, want fallback %q", nested["title"], "Привет")
	}
}

func TestMapOutputToNested_RoundTrip(t *testing.T) {
	raw := []byte(`{
		"language": "en",
		"data": {
			"title": "Варианты работы",
			"phpDev": {
				"title": "PHP разработчик",
				"note": "Только"
			}
		}
	}`)

	dataList, err := MapJSONToDataList("req-rt", raw)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	outputs := []domain.DataOutput{
		{TextHash: hash("en", "title", "Варианты работы"), TranslatedText: "Work options"},
		{TextHash: hash("en", "phpDev.title", "PHP разработчик"), TranslatedText: "PHP developer"},
		{TextHash: hash("en", "phpDev.note", "Только"), TranslatedText: "Only"},
	}

	nested := MapOutputToNested(dataList, outputs)

	if nested["title"] != "Work options" {
		t.Errorf("title: got %q, want %q", nested["title"], "Work options")
	}
	phpDev, ok := nested["phpDev"].(map[string]interface{})
	if !ok {
		t.Fatal("phpDev is not a map")
	}
	if phpDev["title"] != "PHP developer" {
		t.Errorf("phpDev.title: got %q, want %q", phpDev["title"], "PHP developer")
	}
	if phpDev["note"] != "Only" {
		t.Errorf("phpDev.note: got %q, want %q", phpDev["note"], "Only")
	}
}

func TestMapJSONToDataList_NumSequential(t *testing.T) {
	raw := []byte(`{
		"language": "en",
		"data": {
			"a": "one",
			"b": "two",
			"c": "three"
		}
	}`)

	result, err := MapJSONToDataList("req-6", raw)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	nums := make(map[int]bool)
	for _, d := range result.Data {
		if d.Num < 1 || d.Num > 3 {
			t.Errorf("Num %d out of range [1,3]", d.Num)
		}
		if nums[d.Num] {
			t.Errorf("duplicate Num %d", d.Num)
		}
		nums[d.Num] = true
	}
}

func TestMapJSONToDataList_TransJSON(t *testing.T) {
	raw := []byte(`{
		"language": "en",
		"data": {
			"title": "Варианты работы, которые мне подходят",
			"date": "01.03.2026",
			"phpDev": {
				"title": "PHP разработчик удалённо — от $2500 в месяц",
				"note": "Только в англоязычную компанию."
			}
		}
	}`)

	result, err := MapJSONToDataList("req-7", raw)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.Language != "en" {
		t.Errorf("Language: got %q, want %q", result.Language, "en")
	}
	if len(result.Data) != 4 {
		t.Fatalf("Data len: got %d, want 4", len(result.Data))
	}

	expected := map[string]string{
		"title":        "Варианты работы, которые мне подходят",
		"date":         "01.03.2026",
		"phpDev.title": "PHP разработчик удалённо — от $2500 в месяц",
		"phpDev.note":  "Только в англоязычную компанию.",
	}
	for path, value := range expected {
		d, ok := result.Data[path]
		if !ok {
			t.Errorf("key %q not found", path)
			continue
		}
		if d.Text != value {
			t.Errorf("Data[%q].Text: got %q, want %q", path, d.Text, value)
		}
		if d.TextHash != hash("en", path, value) {
			t.Errorf("Data[%q].TextHash mismatch", path)
		}
	}
}
