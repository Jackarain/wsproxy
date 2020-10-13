package websocket

import (
	"io"

	"github.com/gobwas/ws"
)

// Websocket ...
type Websocket struct {
	Conn     *io.ReadWriter
	Encoding string
}

// NewWebsocket ...
func NewWebsocket(conn io.ReadWriter) (*Websocket, error) {
	encoding := ""

	u := ws.Upgrader{
		OnHeader: func(key, value []byte) (err error) {
			if string(key) == "Content-Encoding" {
				encoding = string(value)
			}
			return
		},
	}
	_, err := u.Upgrade(conn)
	if err != nil {
		return nil, err
	}

	return &Websocket{
		Conn:     &conn,
		Encoding: encoding,
	}, nil
}

// ReadMessage ...
func (w *Websocket) ReadMessage() (op ws.OpCode, p []byte, err error) {
	header, err := ws.ReadHeader(*w.Conn)
	if err != nil {
		return 0, nil, err
	}

	payload := make([]byte, header.Length)
	_, err = io.ReadFull(*w.Conn, payload)
	if err != nil {
		return 0, nil, err
	}
	if header.Masked {
		ws.Cipher(payload, header.Mask, 0)
	}

	return header.OpCode, payload, err
}

// WriteMessage ...
func (w *Websocket) WriteMessage(op ws.OpCode, data []byte) error {
	f := ws.NewFrame(op, true, data)
	if err := ws.WriteFrame(*w.Conn, f); err != nil {
		return err
	}

	return nil
}
