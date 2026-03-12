package internal

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"transbroker/internal/cache"
	"transbroker/internal/domain"

	"github.com/nats-io/nats.go"
)

// translateHandler реплицирует логику /translate из main.go.
func translateHandler(w http.ResponseWriter, req *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	resp := NewTransListResponse(
		nil,
		"",
		req.Header.Get("X-Request-Id"),
		req.Body,
		nil,
		nil,
	)
	if resp.StatusCode {
		w.WriteHeader(http.StatusOK)
	} else {
		w.WriteHeader(http.StatusBadRequest)
	}
	_ = json.NewEncoder(w).Encode(resp)
}

// stubProcess — заглушка: добавляет суффикс " [translated]" к каждому тексту.
func stubProcess(_ *nats.Conn, _ string, in *domain.DataList, _ *KafkaConsumerChanList, _ *cache.Cache) ([]domain.DataOutput, error) {
	out := make([]domain.DataOutput, 0, len(in.Data))
	for _, d := range in.Data {
		out = append(out, domain.DataOutput{
			TextHash:       d.TextHash,
			Text:           d.Text,
			TranslatedText: d.Text + " [translated]",
			StatusCode:     true,
		})
	}
	return out, nil
}

func setupTestServer(t *testing.T) *httptest.Server {
	t.Helper()
	orig := processFn
	processFn = stubProcess
	t.Cleanup(func() { processFn = orig })

	mux := http.NewServeMux()
	mux.HandleFunc("/translate", translateHandler)
	return httptest.NewServer(mux)
}

func TestTranslateEndpoint_HappyPath(t *testing.T) {
	srv := setupTestServer(t)
	defer srv.Close()

	body := `{
		"language": "en",
		"data": {
			"title": "Привет",
			"section": {
				"text": "Мир"
			}
		}
	}`

	req, _ := http.NewRequest(http.MethodPost, srv.URL+"/translate", strings.NewReader(body))
	req.Header.Set("X-Request-Id", "test-req-1")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("HTTP status: got %d, want %d", resp.StatusCode, http.StatusOK)
	}

	var result domain.NestedDataOutputResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if !result.StatusCode {
		t.Errorf("StatusCode: got false, want true (errorText: %s)", result.ErrorText)
	}
	if result.Language != "en" {
		t.Errorf("Language: got %q, want %q", result.Language, "en")
	}

	title, ok := result.Data["title"].(string)
	if !ok {
		t.Fatal("Data[\"title\"] is not a string")
	}
	if title != "Привет [translated]" {
		t.Errorf("title: got %q, want %q", title, "Привет [translated]")
	}

	section, ok := result.Data["section"].(map[string]interface{})
	if !ok {
		t.Fatal("Data[\"section\"] is not a map")
	}
	if section["text"] != "Мир [translated]" {
		t.Errorf("section.text: got %q, want %q", section["text"], "Мир [translated]")
	}
}

func TestTranslateEndpoint_MissingRequestId(t *testing.T) {
	srv := setupTestServer(t)
	defer srv.Close()

	body := `{"language":"en","data":{"title":"Hello"}}`
	req, _ := http.NewRequest(http.MethodPost, srv.URL+"/translate", strings.NewReader(body))
	// X-Request-Id намеренно не задан

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("HTTP status: got %d, want %d", resp.StatusCode, http.StatusBadRequest)
	}

	var result domain.NestedDataOutputResponse
	_ = json.NewDecoder(resp.Body).Decode(&result)
	if result.StatusCode {
		t.Error("StatusCode: expected false")
	}
	if result.ErrorText == "" {
		t.Error("ErrorText: expected non-empty")
	}
}

func TestTranslateEndpoint_InvalidJSON(t *testing.T) {
	srv := setupTestServer(t)
	defer srv.Close()

	req, _ := http.NewRequest(http.MethodPost, srv.URL+"/translate", strings.NewReader(`not json`))
	req.Header.Set("X-Request-Id", "test-req-3")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("HTTP status: got %d, want %d", resp.StatusCode, http.StatusBadRequest)
	}

	var result domain.NestedDataOutputResponse
	_ = json.NewDecoder(resp.Body).Decode(&result)
	if result.StatusCode {
		t.Error("StatusCode: expected false")
	}
}

func TestTranslateEndpoint_MissingLanguage(t *testing.T) {
	srv := setupTestServer(t)
	defer srv.Close()

	body := `{"data":{"title":"Hello"}}`
	req, _ := http.NewRequest(http.MethodPost, srv.URL+"/translate", strings.NewReader(body))
	req.Header.Set("X-Request-Id", "test-req-4")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("HTTP status: got %d, want %d", resp.StatusCode, http.StatusBadRequest)
	}
}

func TestTranslateEndpoint_ResponseStructureMatchesInput(t *testing.T) {
	srv := setupTestServer(t)
	defer srv.Close()

	body := `{
		"language": "de",
		"data": {
			"a": "текст A",
			"b": {
				"c": "текст B.C",
				"d": "текст B.D"
			}
		}
	}`

	req, _ := http.NewRequest(http.MethodPost, srv.URL+"/translate", strings.NewReader(body))
	req.Header.Set("X-Request-Id", "test-req-5")

	resp, _ := http.DefaultClient.Do(req)
	defer resp.Body.Close()

	var result domain.NestedDataOutputResponse
	_ = json.NewDecoder(resp.Body).Decode(&result)

	if result.Language != "de" {
		t.Errorf("Language: got %q, want %q", result.Language, "de")
	}

	// Структура ответа должна совпадать со структурой входных данных
	if _, ok := result.Data["a"]; !ok {
		t.Error("expected key \"a\" in Data")
	}
	b, ok := result.Data["b"].(map[string]interface{})
	if !ok {
		t.Fatal("expected \"b\" to be a nested object")
	}
	if _, ok := b["c"]; !ok {
		t.Error("expected key \"b.c\" in Data")
	}
	if _, ok := b["d"]; !ok {
		t.Error("expected key \"b.d\" in Data")
	}
}
