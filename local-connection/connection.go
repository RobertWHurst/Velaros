package localconnection

type Connection struct {
	announceOpenHandlers  []func(string, string)
	announceCloseHandlers []func(string, string)
	dispatchHandlers      map[string]func(string, []byte) bool
}

func New() *Connection {
	return &Connection{
		announceOpenHandlers:  []func(string, string){},
		announceCloseHandlers: []func(string, string){},
		dispatchHandlers:      map[string]func(string, []byte) bool{},
	}
}
