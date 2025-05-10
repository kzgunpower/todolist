package main

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/IBM/sarama"
)

type KafkaEvent struct {
	Timestamp string `json:"timestamp"`
	Action    string `json:"action"`
}

func main() {
	
	logger := log.New(os.Stdout, "[KAFKA-LOGGER] ", log.LstdFlags|log.Lshortfile)

	
	file, err := os.OpenFile("logs.log", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		logger.Fatalf("Failed to open log file: %v", err)
	}
	defer file.Close()

	
	config := sarama.NewConfig()
	config.Consumer.Return.Errors = true

	consumer, err := sarama.NewConsumer([]string{"kafka:9092"}, config)
	if err != nil {
		logger.Fatalf("Failed to create consumer: %v", err)
	}
	defer consumer.Close()

	partitionConsumer, err := consumer.ConsumePartition("logs", 0, sarama.OffsetNewest)
	if err != nil {
		logger.Fatalf("Failed to create partition consumer: %v", err)
	}
	defer partitionConsumer.Close()

	signals := make(chan os.Signal, 1)
	signal.Notify(signals, syscall.SIGINT, syscall.SIGTERM)

	logger.Println("Consumer started. Waiting for messages...")

ConsumerLoop:
	for {
		select {
		case msg := <-partitionConsumer.Messages():
			var event KafkaEvent
			if err := json.Unmarshal(msg.Value, &event); err != nil {
				logger.Printf("Error unmarshaling message: %v", err)
				continue
			}

			logEntry := fmt.Sprintf("[%s] %s - %s\n", 
				time.Now().Format(time.RFC3339), 
				event.Action, 
				event.Timestamp)

			if _, err := file.WriteString(logEntry); err != nil {
				logger.Printf("Error writing to log file: %v", err)
			}

		case err := <-partitionConsumer.Errors():
			logger.Printf("Error from consumer: %v", err)

		case <-signals:
			logger.Println("Shutting down consumer...")
			break ConsumerLoop
		}
	}
	logger.Println("Consumer stopped gracefully")
}