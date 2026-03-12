package domain

import "sync"

type Data struct {
	TextHash string `json:"textHash"`
	Text     string `json:"text"`
	Num      int    `json:"num"`
}

type DataList struct {
	RequestHash string          `json:"requestHash"`
	Language    string          `json:"language"`
	Data        map[string]Data `json:"data"`
}

type PreparedData struct {
	RequestHash    string `json:"requestHash"`
	Language       string `json:"language"`
	TextHash       string `json:"textHash"`
	Text           string `json:"text"`
	TranslatedText string `json:"translatedText"`
	StatusCode     bool   `json:"statusCode"`
	ErrorText      string `json:"errorText"`
}

type PreparedDataList struct {
	Mu       sync.Mutex
	DataList []PreparedData
}

type DataOutput struct {
	TextHash       string `json:"textHash"`
	Text           string `json:"text"`
	TranslatedText string `json:"translatedText"`
	StatusCode     bool   `json:"statusCode"`
	ErrorText      string `json:"errorText"`
}

type DataOutputResponse struct {
	RequestHash string       `json:"requestHash"`
	Data        []DataOutput `json:"data"`
	StatusCode  bool         `json:"statusCode"`
	ErrorText   string       `json:"errorText"`
}

type NestedDataOutputResponse struct {
	Language   string                 `json:"language"`
	Data       map[string]interface{} `json:"data"`
	StatusCode bool                   `json:"statusCode"`
	ErrorText  string                 `json:"errorText"`
}
