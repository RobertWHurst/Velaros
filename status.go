package velaros

import "github.com/coder/websocket"

// Status represents a WebSocket close status code as defined in RFC 6455. Use these
// codes when calling CloseWithStatus to indicate the reason for closing the connection.
type Status = websocket.StatusCode

// WebSocket close status codes
const (
	StatusNormalClosure           Status = websocket.StatusNormalClosure           // 1000
	StatusGoingAway               Status = websocket.StatusGoingAway               // 1001
	StatusProtocolError           Status = websocket.StatusProtocolError           // 1002
	StatusUnsupportedData         Status = websocket.StatusUnsupportedData         // 1003
	StatusNoStatusRcvd            Status = websocket.StatusNoStatusRcvd            // 1005
	StatusAbnormalClosure         Status = websocket.StatusAbnormalClosure         // 1006
	StatusInvalidFramePayloadData Status = websocket.StatusInvalidFramePayloadData // 1007
	StatusPolicyViolation         Status = websocket.StatusPolicyViolation         // 1008
	StatusMessageTooBig           Status = websocket.StatusMessageTooBig           // 1009
	StatusMandatoryExtension      Status = websocket.StatusMandatoryExtension      // 1010
	StatusInternalError           Status = websocket.StatusInternalError           // 1011
	StatusServiceRestart          Status = websocket.StatusServiceRestart          // 1012
	StatusTryAgainLater           Status = websocket.StatusTryAgainLater           // 1013
	StatusBadGateway              Status = websocket.StatusBadGateway              // 1014
	StatusTLSHandshake            Status = websocket.StatusTLSHandshake            // 1015
)

// CloseSource indicates whether a connection close was initiated by the client or server.
// Use CloseStatus() in UseClose handlers to check who initiated the close and respond
// accordingly.
type CloseSource int

const (
	ClientCloseSource CloseSource = iota
	ServerCloseSource
)
