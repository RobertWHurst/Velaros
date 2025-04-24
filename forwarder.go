package velaros

// Forwarder is an interface similar to the interplexer, but is designed for use
// with frameworks such as Eurus, which holds the connection at a gateway, then
// creates a velaros context with a socket instance that represents the socket
// at the gateway. Due to the need to forward socket state to the gateway,
// valeros provides this Forwarder interface to allow frameworks like Eurus to
// forward messages to the gateway to track the socket state.
type Forwarder interface {
	SetOnSocket(socketID string, key string, value any)
	GetFromSocket(socketID string, key string) any
	WithSocket(targetSocketID string, messageDecoder MessageDecoder, messageEncoder MessageEncoder) (SocketHandle, bool)
}
