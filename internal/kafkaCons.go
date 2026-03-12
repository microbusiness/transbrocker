package internal

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/confluentinc/confluent-kafka-go/kafka"
)

type KafkaConsumer struct {
	Consumer     *kafka.Consumer
	ConsumerAddr string
}

type KafkaConsumerChanList struct {
	mu       sync.Mutex
	ChanList map[ReqTextKey]chan []byte
}

type ReqTextKey struct {
	RequestHash string
	TextHash    string
}

func NewKafkaConsumer(addr string, topic string) (*KafkaConsumer, error) {
	c, err := kafka.NewConsumer(&kafka.ConfigMap{
		"bootstrap.servers": addr,
		"group.id":          topic,
		"auto.offset.reset": "earliest",
	})
	if err != nil {
		panic(err)
	}

	return &KafkaConsumer{
		Consumer:     c,
		ConsumerAddr: addr,
	}, nil

}

func NewKafkaConsumerChanList() *KafkaConsumerChanList {
	return &KafkaConsumerChanList{
		ChanList: make(map[ReqTextKey]chan []byte),
	}
}

func (c *KafkaConsumer) Subscribe(topic string, ch *KafkaConsumerChanList) error {
	err := c.CreateTopic(topic)
	if err != nil {
		return fmt.Errorf("failed to create topic: %w", err)
	}

	err = c.Consumer.SubscribeTopics([]string{topic}, nil)
	if err != nil {
		return err
	}

	for {
		msg, err := c.Consumer.ReadMessage(-1)
		if err == nil {
			reqTextKey := ReqTextKey{}
			for _, header := range msg.Headers {
				if string(header.Key) == "reqId" {
					reqTextKey.RequestHash = string(header.Value)
				}
				if string(header.Key) == "textId" {
					reqTextKey.TextHash = string(header.Value)
				}
			}
			if reqTextKey.RequestHash != "" && reqTextKey.TextHash != "" {
				if _, ok := ch.ChanList[reqTextKey]; ok {
					ch.ChanList[reqTextKey] <- msg.Value
					close(ch.ChanList[reqTextKey])
				}
			}
		} else {
			fmt.Printf("Ошибка консьюмера: %v (%v)\n", err, msg)
		}
	}
}

func (c *KafkaConsumer) CreateTopic(topic string) error {
	a, err := kafka.NewAdminClient(&kafka.ConfigMap{"bootstrap.servers": c.ConsumerAddr})
	if err != nil {
		panic(err)
	}
	defer a.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	metadata, err := a.GetMetadata(&topic, false, 10000)
	if err != nil {
		return fmt.Errorf("failed to get metadata: %w", err)
	}

	// Check if the specific topic name exists in the map of topics
	if _, ok := metadata.Topics[topic]; ok {
		return nil
	}

	_, err = a.CreateTopics(ctx, []kafka.TopicSpecification{{
		Topic: topic, NumPartitions: 1, ReplicationFactor: 1,
	}})
	if err != nil {
		return fmt.Errorf("failed to create topics: %w", err)
	}
	return nil
}

func (ch *KafkaConsumerChanList) AddChan(key ReqTextKey) {
	ch.mu.Lock()
	defer ch.mu.Unlock()
	ch.ChanList[key] = make(chan []byte)
}
