package internal

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"
	"transbroker/internal/domain"
)

type transRequest struct {
	Language string                 `json:"language"`
	Data     map[string]interface{} `json:"data"`
}

// MapJSONToDataList maps a nested JSON (like client/trans.json) to domain.DataList.
// Expected format: { "language": "en", "data": { ... } }
// All string leaf values are extracted with their dot-separated path as key.
// TextHash = SHA256(language + ":" + path + ":" + value).
func MapJSONToDataList(requestHash string, rawJSON []byte) (domain.DataList, error) {
	var req transRequest
	if err := json.Unmarshal(rawJSON, &req); err != nil {
		return domain.DataList{}, fmt.Errorf("error unmarshalling JSON: %w", err)
	}
	if req.Language == "" {
		return domain.DataList{}, fmt.Errorf("missing \"language\" field in JSON")
	}

	dataList := domain.DataList{
		RequestHash: requestHash,
		Language:    req.Language,
		Data:        make(map[string]domain.Data),
	}

	num := 0
	flattenMap(req.Data, "", req.Language, &dataList, &num)

	return dataList, nil
}

// flattenMap recursively walks obj, building dot-separated paths for each string leaf.
func flattenMap(obj map[string]interface{}, prefix string, language string, dataList *domain.DataList, num *int) {
	for key, val := range obj {
		path := key
		if prefix != "" {
			path = prefix + "." + key
		}

		switch v := val.(type) {
		case string:
			*num++
			dataList.Data[path] = domain.Data{
				TextHash: calcTextHash(language, path, v),
				Text:     v,
				Num:      *num,
			}
		case map[string]interface{}:
			flattenMap(v, path, language, dataList, num)
		}
	}
}

// calcTextHash returns SHA256 hex of "language:path:value".
func calcTextHash(language, path, value string) string {
	sum := sha256.Sum256([]byte(language + ":" + path + ":" + value))
	return hex.EncodeToString(sum[:])
}

// MapOutputToNested rebuilds the original nested JSON structure,
// replacing each leaf value with TranslatedText from outputs.
// Falls back to the original Text if a translation is missing.
func MapOutputToNested(dataList domain.DataList, outputs []domain.DataOutput) map[string]interface{} {
	translated := make(map[string]string, len(outputs))
	for _, out := range outputs {
		translated[out.TextHash] = out.TranslatedText
	}

	nested := make(map[string]interface{})
	for path, data := range dataList.Data {
		text := data.Text
		if t, ok := translated[data.TextHash]; ok && t != "" {
			text = t
		}
		setNested(nested, strings.Split(path, "."), text)
	}
	return nested
}

// setNested inserts value into obj at the location described by parts.
func setNested(obj map[string]interface{}, parts []string, value string) {
	if len(parts) == 1 {
		obj[parts[0]] = value
		return
	}
	key := parts[0]
	if _, ok := obj[key]; !ok {
		obj[key] = make(map[string]interface{})
	}
	setNested(obj[key].(map[string]interface{}), parts[1:], value)
}
