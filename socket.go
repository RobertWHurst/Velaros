package scramjet

type Socket struct {
}

func (s *Socket) Send(path string, payload any) {
}

func (s *Socket) Request(path string, payload any) *Message {
	return &Message{}
}
