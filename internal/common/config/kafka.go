package config

import "time"

type KafkaProducerMode string

const (
	KafkaProducerModeDirect KafkaProducerMode = "direct"
	KafkaProducerModeOutbox KafkaProducerMode = "outbox"
)

type KafkaTopic struct {
	Brokers []string `yaml:"brokers" validate:"required,min=1,dive,hostname_port"`
	Topic   string   `yaml:"topic" validate:"required"`
}

type Kafka struct {
	Enabled bool `yaml:"enabled" validate:"required"`

	LinkUpdates KafkaTopic `yaml:"link_updates" validate:"required"`
	FailedLinks KafkaTopic `yaml:"failed_links" validate:"required"`
	DLQ         KafkaTopic `yaml:"dlq"`

	SchemaRegistryURL string `yaml:"schema_registry_url"`
	UpdateSubject     string `yaml:"update_subject"` // link-updates-value
	FailedSubject     string `yaml:"failed_subject"` // failed-links-value
}

type KafkaConsumer struct {
	ConsumerGroup  string `yaml:"consumer_group" validate:"required"`
	ConsumerClient string `yaml:"consumer_client_id" validate:"required"`

	ReadTimeout    time.Duration `yaml:"read_timeout" validate:"required,gt=0"`
	CommitInterval time.Duration `yaml:"commit_interval" validate:"required,gte=0"`
	StartOffset    string        `yaml:"start_offset" validate:"required,oneof=earliest latest"`

	ProcessRetries int           `yaml:"process_retries" validate:"required,gte=1,lte=20"`
	RetryDelay     time.Duration `yaml:"retry_delay" validate:"required,gt=0"`

	TopicWorkers int `yaml:"topic_workers" validate:"gte=0,lte=32"`
}

type KafkaProducer struct {
	Mode KafkaProducerMode `yaml:"mode"`

	ProducerClient string `yaml:"producer_client"`

	WriteTimeout time.Duration `yaml:"write_timeout" validate:"required,gt=0"`
	RequiredACK  int           `yaml:"required_ack" validate:"required,oneof=-1 1 0"`
	MaxAttempts  int           `yaml:"max_attempts" validate:"required,gte=1,lte=20"`
}

type KafkaOutbox struct {
	BatchSize    int           `yaml:"batch_size" validate:"required,gte=1"`
	PollInterval time.Duration `yaml:"poll_interval" validate:"required,gt=0"`
	LockFor      time.Duration `yaml:"lock_for" validate:"required,gt=0"`
	MaxAttempts  int           `yaml:"max_attempts" validate:"required,gte=1"`
	BaseBackoff  time.Duration `yaml:"base_backoff" validate:"required,gt=0"`
	MaxBackoff   time.Duration `yaml:"max_backoff" validate:"required,gt=0"`
}
