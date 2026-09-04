package kafka

type Message struct {
	Topic   string
	Key     string
	Value   []byte
	Headers map[string]string
}
