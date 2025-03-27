package localconnection

import "github.com/RobertWHurst/velaros"

type Connection struct {
	announceOpenHandlers  []func(string, string)
	announceCloseHandlers []func(string, string)
	dispatchHandlers      map[string]func(string, []byte) bool
}

func New() velaros.InterplexerConnection {
	return &Connection{
		announceOpenHandlers:  []func(string, string){},
		announceCloseHandlers: []func(string, string){},
		dispatchHandlers:      map[string]func(string, []byte) bool{},
	}
}
