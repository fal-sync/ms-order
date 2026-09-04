package kafka

import (
	"context"

	kafkago "github.com/segmentio/kafka-go"
)

type Writer struct {
	writer *kafkago.Writer
}

func NewWriter(brokers []string) *Writer {
	return &Writer{
		writer: &kafkago.Writer{
			Addr:     kafkago.TCP(brokers...),
			Balancer: &kafkago.LeastBytes{},
		},
	}
}

func (w *Writer) Publish(ctx context.Context, message Message) error {
	headers := make([]kafkago.Header, 0, len(message.Headers))
	for key, value := range message.Headers {
		headers = append(headers, kafkago.Header{
			Key:   key,
			Value: []byte(value),
		})
	}

	return w.writer.WriteMessages(ctx, kafkago.Message{
		Topic:   message.Topic,
		Key:     []byte(message.Key),
		Value:   message.Value,
		Headers: headers,
	})
}

func (w *Writer) Close() error {
	return w.writer.Close()
}
