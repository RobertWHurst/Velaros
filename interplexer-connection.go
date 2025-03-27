package velaros

type InterplexerConnection interface {
	AnnounceSocketOpen(interplexerID string, socketID string) error
	BindSocketOpenAnnounce(handler func(interplexerID string, socketID string)) error
	UnbindSocketOpenAnnounce() error

	AnnounceSocketClose(interplexerID string, socketID string) error
	BindSocketCloseAnnounce(handler func(interplexerID string, socketID string)) error
	UnbindSocketCloseAnnounce() error

	Dispatch(interplexerID string, socketID string, message []byte) error
	BindDispatch(interplexerID string, handler func(socketID string, message []byte) bool) error
	UnbindDispatch(interplexerID string) error
}
