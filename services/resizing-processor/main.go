package main

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/zhunismp/imagep-backend/internal/kafka"
	"github.com/zhunismp/imagep-backend/internal/models"
)

func main() {
	ctx, cancel := context.WithCancel(context.Background())
	kafkaAddr := "localhost:9092"
	if err := kafka.PrepareTopic(ctx, "development", kafkaAddr); err != nil {
		log.Fatalf("Failed to create topic: %v", err)
	}

	group := "cg-resize-image"
	topic := "process-image"
	ch := make(chan models.ProcessImageMessage, 100)
	defer cancel()

	consumer, err := kafka.NewKafkaConsumer(
		kafkaAddr,
		group,
		topic,
		ch,
	)
	if err != nil {
		log.Fatalf("Failed to initialize consumer: %v", err)
	}

	consumer.WaitReady(10 * time.Second)
	consumer.Start(ctx)

	for msg := range ch {
		fmt.Printf("message received: %s", msg)
	}
}
