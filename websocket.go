package graphql

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"

	"github.com/gorilla/websocket"
)

func newWebsocketConnection(ctx context.Context, u string, header http.Header) (*websocket.Conn, error) {
	wsURL, err := url.Parse(u)
	if err != nil {
		return nil, err
	}
	if wsURL.Scheme == "https" || wsURL.Scheme == "wss" {
		wsURL.Scheme = "wss"
	} else {
		wsURL.Scheme = "ws"
	}

	d := websocket.DefaultDialer
	d.Subprotocols = []string{"graphql-ws"}
	conn, _, err := d.DialContext(ctx, wsURL.String(), header)
	if err != nil {
		return nil, err
	}

	if err := conn.WriteMessage(websocket.TextMessage, []byte(`{"type":"connection_init","payload":{}}`)); err != nil {
		return nil, err
	}

	_, body, err := conn.ReadMessage()
	if err != nil {
		return nil, err
	}
	if string(body) != `{"type":"connection_ack"}`+"\n" {
		return nil, fmt.Errorf("Did not receive ack, got: %s'", string(body))
	}

	_, body, err = conn.ReadMessage()
	if err != nil {
		return nil, err
	}
	if string(body) != `{"type":"ka"}`+"\n" {
		return nil, fmt.Errorf("Did not receive ka, got: %s'", string(body))
	}

	return conn, nil
}

type GraphQLPayload struct {
	Variables     map[string]interface{} `json:"variables"`
	Extensions    map[string]interface{} `json:"extensions"`
	OperationName *string                `json:"operationName"`
	Query         string                 `json:"query"`
}

type SubscriptionAction string

const (
	START    SubscriptionAction = "start"
	STOP     SubscriptionAction = "stop"
	DATA     SubscriptionAction = "data"
	ERROR    SubscriptionAction = "error"
	COMPLETE SubscriptionAction = "complete"
)

type Subscription struct {
	ID      string             `json:"id"`
	Type    SubscriptionAction `json:"type"`
	Payload GraphQLPayload     `json:"payload"`
}

type SubscriptionMessage struct {
	ID      string             `json:"id"`
	Type    SubscriptionAction `json:"type"`
	Payload struct {
		Data json.RawMessage `json:"data"`
	} `json:"payload"`
	Errors json.RawMessage `json:"errors"`
}
