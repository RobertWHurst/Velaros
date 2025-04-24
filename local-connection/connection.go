package localconnection

import (
	"time"

	"github.com/RobertWHurst/velaros"
)

type Connection struct {
	announceOpenHandlers  []func(string, string)
	announceCloseHandlers []func(string, string)
	dispatchHandlers      map[string]func(string, []byte)
	interceptHandlers     map[string]func(string, string, time.Duration)
	ignoreHandlers        map[string]func(string, string)
	interceptedHandlers   map[string]func(string, string, []byte)
}

func New() velaros.InterplexerConnection {
	return &Connection{
		announceOpenHandlers:  []func(string, string){},
		announceCloseHandlers: []func(string, string){},
		dispatchHandlers:      map[string]func(string, []byte){},
		interceptHandlers:     map[string]func(string, string, time.Duration){},
		ignoreHandlers:        map[string]func(string, string){},
		interceptedHandlers:   map[string]func(string, string, []byte){},
	}
}
