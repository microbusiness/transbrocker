package internal

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"sync"
	"time"
	"transbroker/internal/cache"
	"transbroker/internal/domain"

	"github.com/nats-io/nats.go"
)

func Process(
	nc *nats.Conn,
	subject string,
	in *domain.DataList,
	ch *KafkaConsumerChanList,
	transCache *cache.Cache) ([]domain.DataOutput, error) {

	preparedDataList := domain.PreparedDataList{
		DataList: make([]domain.PreparedData, 0),
	}

	var wg sync.WaitGroup
	for _, inData := range in.Data {
		if preparedData, ok := transCache.GetTrans(cache.TransIdent{
			Language: in.Language,
			TextHash: inData.TextHash,
		}); ok {
			preparedDataList.DataList = append(preparedDataList.DataList, preparedData)
			continue
		}

		preparedData := domain.PreparedData{
			RequestHash: in.RequestHash,
			Language:    in.Language,
			TextHash:    inData.TextHash,
			Text:        inData.Text,
		}
		jsonPreparedData, errJson := json.Marshal(preparedData)
		if errJson != nil {
			log.Printf("Error marshalling preparedData: %v", errJson)
			preparedData.TranslatedText = inData.Text
			preparedData.StatusCode = false
			preparedData.ErrorText = fmt.Sprintf("Error marshalling preparedData: %v", errJson)
			preparedDataList.Mu.Lock()
			preparedDataList.DataList = append(preparedDataList.DataList, preparedData)
			preparedDataList.Mu.Unlock()
			continue
		}
		wg.Add(1)
		ch.mu.Lock()
		ch.AddChan(ReqTextKey{
			RequestHash: in.RequestHash,
			TextHash:    inData.TextHash,
		})
		ch.mu.Unlock()

		go func(data []byte) {
			defer wg.Done()
			errPub := nc.Publish(subject, data)
			if errPub != nil {
				log.Printf("Error publishing message: %v", errPub)
				preparedData.TranslatedText = inData.Text
				preparedData.StatusCode = false
				preparedData.ErrorText = fmt.Sprintf("Error publishing message: %v", errJson)
				preparedDataList.Mu.Lock()
				preparedDataList.DataList = append(preparedDataList.DataList, preparedData)
				preparedDataList.Mu.Unlock()
				return
			}
			ch.mu.Lock()
			currChan := ch.ChanList[ReqTextKey{
				RequestHash: in.RequestHash,
				TextHash:    inData.TextHash,
			}]
			ch.mu.Unlock()

			select {
			case res := <-currChan:
				var dataOut domain.PreparedData
				errJs := json.Unmarshal(res, &dataOut)
				if errJs != nil {
					log.Printf("error unmarshalling JSON: %v", errJs)
					preparedData.TranslatedText = inData.Text
					preparedData.StatusCode = false
					preparedData.ErrorText = fmt.Sprintf("error unmarshalling JSON from Kafka: %v", errJson)
					preparedDataList.Mu.Lock()
					preparedDataList.DataList = append(preparedDataList.DataList, preparedData)
					preparedDataList.Mu.Unlock()
				} else {
					preparedDataList.Mu.Lock()
					preparedDataList.DataList = append(preparedDataList.DataList, dataOut)
					transCache.SetTrans(cache.TransIdent{
						Language: in.Language,
						TextHash: inData.TextHash,
					}, dataOut)
					preparedDataList.Mu.Unlock()
				}
			case <-time.After(120 * time.Second):
				preparedData.TranslatedText = inData.Text
				preparedData.StatusCode = false
				preparedData.ErrorText = fmt.Sprintf("error: Время истекло (timeout) Kafka: %v", errJson)
				preparedDataList.Mu.Lock()
				preparedDataList.DataList = append(preparedDataList.DataList, preparedData)
				preparedDataList.Mu.Unlock()

			}
			ch.mu.Lock()
			delete(ch.ChanList, ReqTextKey{
				RequestHash: in.RequestHash,
				TextHash:    inData.TextHash,
			})
			ch.mu.Unlock()
			return
		}(jsonPreparedData)

	}
	wg.Wait()

	return mapToOut(&preparedDataList), nil
}

func mapToOut(list *domain.PreparedDataList) []domain.DataOutput {
	result := make([]domain.DataOutput, 0)
	for _, data := range list.DataList {
		result = append(result, domain.DataOutput{
			TextHash:       data.TextHash,
			Text:           data.Text,
			TranslatedText: data.TranslatedText,
			StatusCode:     data.StatusCode,
			ErrorText:      data.ErrorText,
		})
	}
	return result
}

// processFn can be replaced in tests to stub Process.
var processFn = Process

func NewTransListResponse(
	nc *nats.Conn,
	natsSubject string,
	requestId string,
	reqBody io.Reader,
	chanRequests *KafkaConsumerChanList,
	transCache *cache.Cache) domain.NestedDataOutputResponse {
	transListResp := domain.NestedDataOutputResponse{
		Language:   "",
		Data:       make(map[string]interface{}),
		StatusCode: true,
		ErrorText:  "",
	}

	if requestId == "" {
		transListResp.StatusCode = false
		transListResp.ErrorText = "Don't have X-Request-Id"
	}

	var in domain.DataList

	if transListResp.StatusCode {
		rawBody, errRead := io.ReadAll(reqBody)
		if errRead != nil {
			log.Printf("Error reading request body: %v", errRead)
			transListResp.StatusCode = false
			transListResp.ErrorText = fmt.Sprintf("Error reading request body: %v", errRead)
		} else {
			var errMap error
			in, errMap = MapJSONToDataList(requestId, rawBody)
			if errMap != nil {
				log.Printf("Error mapping request JSON: %v", errMap)
				transListResp.StatusCode = false
				transListResp.ErrorText = fmt.Sprintf("Error mapping request JSON: %v", errMap)
			}
		}
	}

	transListResp.Language = in.Language

	if transListResp.StatusCode {
		outputs, errProc := processFn(nc, natsSubject, &in, chanRequests, transCache)
		if errProc != nil {
			fmt.Println(errProc)
		}
		transListResp.Data = MapOutputToNested(in, outputs)
	}
	return transListResp
}
