package localconnection

import "github.com/RobertWHurst/scramjet"

type Connection struct {
	announceOpenHandlers  []func(string, string)
	announceCloseHandlers []func(string, string)
	dispatchHandlers      map[string]func(string, []byte) bool
}

func New() scramjet.InterplexerConnection {
	return &Connection{
		announceOpenHandlers:  []func(string, string){},
		announceCloseHandlers: []func(string, string){},
		dispatchHandlers:      map[string]func(string, []byte) bool{},
	}
}
