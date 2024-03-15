package scramjet

type Message struct {
	socket         *Socket
	path           string
	associatedData map[string]any
	rawPayload     []byte
	payload        any
	responsePath   string

	handlerChain *HandlerChain
}

func (m *Message) Socket() *Socket {
	return m.socket
}

func (m *Message) Path() string {
	return m.path
}

func (m *Message) Set(key string, value any) {
	m.associatedData[key] = value
}

func (m *Message) Get(key string) any {
	return m.associatedData[key]
}

func (m *Message) Payload() any {
	// TODO: lazy decode payload
	return m.payload
}

func (m *Message) PayloadBytes() []byte {
	return m.rawPayload
}

func (m *Message) SocketByID(id string) *Socket {
	return &Socket{}
}

func (m *Message) Send(payload any) {
	m.socket.Send(m.responsePath, payload)
}

func (m *Message) Request(payload any) *Message {
	return m.socket.Request(m.responsePath, payload)
}

func (m *Message) Next() {

}
