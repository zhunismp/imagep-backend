package kafka

import (
	"context"
	"errors"
	"fmt"

	"github.com/confluentinc/confluent-kafka-go/v2/kafka"
)

// This function will activate only development environment
// For production environment, the topic creation happen before application is running
func PrepareTopic(ctx context.Context, env string, kafkaAddr string) error {

	if env != "development" {
		return nil
	}

	admin, err := kafka.NewAdminClient(&kafka.ConfigMap{"bootstrap.servers": kafkaAddr})
	if err != nil {
		return fmt.Errorf("error while creating admin: %v", err)
	}

	res, err := admin.CreateTopics(ctx, []kafka.TopicSpecification{{
		Topic:             "process-image",
		NumPartitions:     3,
		ReplicationFactor: 1, // 1 node for dev
	}})

	for _, r := range res {
		if r.Error.Code() != kafka.ErrNoError {
			errors.Join(err, fmt.Errorf("result for topic %s: %v", r.Topic, r.Error))
		}
	}

	return err
}
