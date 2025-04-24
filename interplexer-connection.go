package velaros

import "time"

type InterplexerConnection interface {
	AnnounceSocketOpen(interplexerID string, socketID string) error
	BindSocketOpenAnnounce(handler func(interplexerID string, socketID string)) error
	UnbindSocketOpenAnnounce() error

	AnnounceSocketClose(interplexerID string, socketID string) error
	BindSocketCloseAnnounce(handler func(interplexerID string, socketID string)) error
	UnbindSocketCloseAnnounce() error

	Dispatch(interplexerID string, socketID string, message []byte) error
	BindDispatch(interplexerID string, handler func(socketID string, message []byte)) error
	UnbindDispatch(interplexerID string) error

	Intercept(interplexerID string, socketID string, messageID string, timeout time.Duration) error
	BindIntercept(interplexerID string, handler func(socketID string, messageID string, timeout time.Duration)) error
	UnbindIntercept(interplexerID string) error

	Ignore(interplexerID string, socketID string, messageID string) error
	BindIgnore(interplexerID string, handler func(socketID string, messageID string)) error
	UnbindIgnore(interplexerID string) error

	Intercepted(interplexerID string, socketID string, messageID string, message []byte) error
	BindIntercepted(interplexerID string, handler func(socketID string, messageID string, message []byte)) error
	UnbindIntercepted(interplexerID string) error
}
