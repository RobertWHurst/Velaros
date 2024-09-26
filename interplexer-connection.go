package scramjet

type InterplexerConnection interface {
	AnnounceSocketOpen(interplexerID string, socketID string) error
	BindSocketOpenAnnounce(handler func(interplexerID string, socketID string)) error
	UnbindSocketOpenAnnounce()

	AnnounceSocketClose(interplexerID string, socketID string) error
	BindSocketCloseAnnounce(handler func(interplexerID string, socketID string)) error
	UnbindSocketCloseAnnounce()

	Dispatch(interplexerID string, socketID string, message []byte) error
	BindDispatch(interplexerID string, handler func(socketID string, message []byte) bool) error
	UnbindDispatch(interplexerID string)
}
