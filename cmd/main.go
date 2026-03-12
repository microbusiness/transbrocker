package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
	"transbroker/config"
	"transbroker/internal"
	"transbroker/internal/cache"

	"github.com/nats-io/nats.go"
	"github.com/rs/zerolog"
)

func main() {

	zerolog.TimeFieldFormat = zerolog.TimeFormatUnix

	cfg, err := config.New()
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
	fmt.Println(cfg)
	nc, err := nats.Connect(cfg.NatsUrl)
	if err != nil {
		log.Fatalf("Error connecting to NATS server: %v", err)
	}

	defer nc.Drain()

	cons, err := internal.NewKafkaConsumer(cfg.KafkaUrl, cfg.KafkaTopic)
	if err != nil {
		log.Fatalf("Error creating Kafka consumer: %v", err)
	}
	defer cons.Consumer.Close()

	chanRequests := internal.NewKafkaConsumerChanList()

	go func() {
		errSub := cons.Subscribe(cfg.KafkaTopic, chanRequests)
		if errSub != nil {
			log.Fatalf("Error subscribe of Kafka broker: %v", err)
		}
		return
	}()

	log.Print(fmt.Sprintf("Connecting to Kafka broker at %s and subscribed %s", cfg.KafkaUrl, cfg.KafkaTopic))

	transCache := cache.NewCache(cfg.CacheMaxCount, cfg.CleanupInterval)

	startedAt := time.Now()

	http.HandleFunc("/health", func(w http.ResponseWriter, req *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]string{
			"status": "ok",
			"uptime": time.Since(startedAt).Truncate(time.Second).String(),
		})
	})

	http.HandleFunc("/translate", func(w http.ResponseWriter, req *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "http://localhost") // Be specific
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
		w.Header().Set("Content-Type", "application/json")

		transListResp := internal.NewTransListResponse(
			nc,
			cfg.NatsSubject,
			req.Header.Get("X-Request-Id"),
			req.Body,
			chanRequests,
			transCache)

		if transListResp.StatusCode {
			w.WriteHeader(http.StatusOK)
		}
		if !transListResp.StatusCode {
			w.WriteHeader(http.StatusBadRequest)
		}

		errEnc := json.NewEncoder(w).Encode(transListResp)
		if errEnc != nil {
			fmt.Println(errEnc)
		}
	})
	httpUrl := fmt.Sprintf("%s%s", cfg.HttpAddr, cfg.HttpPort)
	errHttp := http.ListenAndServe(httpUrl, nil)
	if errHttp != nil {
		panic(err)
	}

	signalChan := make(chan os.Signal, 1)
	signal.Notify(signalChan, syscall.SIGINT, syscall.SIGTERM)
	<-signalChan

	fmt.Println("\nReceived interrupt signal, draining connection and exiting.")

}
