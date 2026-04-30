package config

import "time"

type Kafka struct {
	Enabled bool `yaml:"enabled" validate:"required"`

	Brokers []string `yaml:"brokers" validate:"required,min=1,dive,hostname_port"`

	UpadateLinksTopic string `yaml:"links_topic" validate:"required"`
	FailedLinksTopic  string `yaml:"failed_links_topic" validate:"required"`
	DLQTopic          string `yaml:"dlq_topic" validate:"required"`

	SchemaRegistryURL string `yaml:"schema_registry_url"`
	UpdateSubject     string `yaml:"update_subject"` // link-updates-value
	FailedSubject     string `yaml:"failed_subject"` // failed-links-value
}

type KafkaConsumer struct {
	ConsumerGroup    string `yaml:"consumer_group" validate:"required"`
	ConsumerClientID string `yaml:"consumer_client_id" validate:"required"`

	ReadTimeout    time.Duration `yaml:"read_timeout" validate:"required,gt=0"`
	CommitInterval time.Duration `yaml:"commit_interval" validate:"required,gte=0"`
	StartOffset    string        `yaml:"start_offset" validate:"required,oneof=earliest latest"`
}

type KafkaProducer struct {
	ProducerClient string `yaml:"producer_client"`

	WriteTimeout time.Duration `yaml:"write_timeout" validate:"required,gt=0"`
	RequiredACK  int           `yaml:"required_ack" validate:"required,oneof=-1 1 0"`
	MaxAttempts  int           `yaml:"max_attempts" validate:"required,gte=1,lte=20"`
}
