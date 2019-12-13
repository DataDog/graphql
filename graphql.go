package graphql

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"reflect"

	"github.com/gorilla/websocket"
	uuid "github.com/satori/go.uuid"
	"github.com/shurcooL/graphql/internal/jsonutil"
	"golang.org/x/net/context/ctxhttp"
)

// Client is a GraphQL client.
type Client struct {
	url        string // GraphQL server URL.
	httpClient *http.Client
	header     http.Header
}

// NewClient creates a GraphQL client targeting the specified GraphQL server URL.
// If httpClient is nil, then http.DefaultClient is used.
func NewClient(url string, httpClient *http.Client) *Client {
	if httpClient == nil {
		httpClient = http.DefaultClient
	}
	return &Client{
		url:        url,
		httpClient: httpClient,
	}
}

// NewClientWithHeader creates a GraphQL client targeting the specified GraphQL server URL and
// a header to be applied to each request.
// If httpClient is nil, then http.DefaultClient is used.
func NewClientWithHeader(url string, httpClient *http.Client, header http.Header) *Client {
	if httpClient == nil {
		httpClient = http.DefaultClient
	}
	return &Client{
		url:        url,
		httpClient: httpClient,
		header:     header,
	}
}

// Query executes a single GraphQL query request,
// with a query derived from q, populating the response into it.
// q should be a pointer to struct that corresponds to the GraphQL schema.
func (c *Client) Query(ctx context.Context, q interface{}, variables map[string]interface{}) error {
	return c.do(ctx, queryOperation, q, variables)
}

// Mutate executes a single GraphQL mutation request,
// with a mutation derived from m, populating the response into it.
// m should be a pointer to struct that corresponds to the GraphQL schema.
func (c *Client) Mutate(ctx context.Context, m interface{}, variables map[string]interface{}) error {
	return c.do(ctx, mutationOperation, m, variables)
}

// Subscribe executes a single GraphQL subscription request,
// with a subscription derived from m, populating the response into it.
func (c *Client) Subscribe(ctx context.Context, m interface{}, variables map[string]interface{}) (chan interface{}, context.CancelFunc, error) {
	ctx, cancel := context.WithCancel(ctx)

	conn, err := newWebsocketConnection(ctx, c.url, c.header)
	if err != nil {
		return nil, cancel, err
	}

	newChan := make(chan interface{}, 1)
	q := constructSubscription(m, variables)
	cancel, err = subJSON(conn, q, variables)
	if err != nil {
		return nil, cancel, err
	}

	val := reflect.ValueOf(m)
	if val.Kind() == reflect.Ptr {
		val = val.Elem()
	}

	go func() {
		for {
			msg := SubscriptionMessage{}
			if err := conn.ReadJSON(&msg); err != nil {
				log.Println(err)
				continue
			}
			// TODO(jlegrone): figure out when this happens -- so far have seen it with "ka" messages.
			if msg.ID == "" {
				continue
			}
			id, err := uuid.FromString(msg.ID)
			if err != nil {
				log.Println(err)
				continue
			}

			switch msg.Type {
			case DATA:
				queryData := reflect.New(val.Type()).Interface()
				if err := jsonutil.UnmarshalGraphQL(msg.Payload.Data, queryData); err == nil {
					newChan <- queryData
				} else {
					log.Printf("Unable to unmarshal graphql for message %s: %s\n", msg.Payload, err)
				}
			case COMPLETE:
				close(newChan)
				break
			case STOP:
				close(newChan)
				break
			default:
				log.Println(id, msg.Type, string(msg.Payload.Data))
				continue
			}
		}
	}()

	return newChan, cancel, nil
}

// subJSON enables you to provide your query in a string format.
// Values passed on the return chan contain a JSON representation of
// the "data" attribute returned by the GraphQL server
func subJSON(conn *websocket.Conn, subscription string, variables map[string]interface{}) (context.CancelFunc, error) {
	id := uuid.NewV4()
	sub := Subscription{
		ID:   id.String(),
		Type: START,
		Payload: GraphQLPayload{
			Query:     subscription,
			Variables: variables,
		},
	}

	if err := conn.WriteJSON(sub); err != nil {
		return nil, err
	}

	ctx, cancel := context.WithCancel(context.Background())

	go func() {
		<-ctx.Done()
		sub.Type = STOP
	}()

	return cancel, nil
}

// do executes a single GraphQL operation.
func (c *Client) do(ctx context.Context, op operationType, v interface{}, variables map[string]interface{}) error {
	var query string
	switch op {
	case queryOperation:
		query = constructQuery(v, variables)
	case mutationOperation:
		query = constructMutation(v, variables)
	case subscriptionOperation:
		query = constructSubscription(v, variables)
	}
	in := struct {
		Query     string                 `json:"query"`
		Variables map[string]interface{} `json:"variables,omitempty"`
	}{
		Query:     query,
		Variables: variables,
	}
	var buf bytes.Buffer
	err := json.NewEncoder(&buf).Encode(in)
	if err != nil {
		return err
	}

	resp, err := ctxhttp.Post(ctx, c.httpClient, c.url, "application/json", &buf)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := ioutil.ReadAll(resp.Body)
		return fmt.Errorf("non-200 OK status code: %v body: %q", resp.Status, body)
	}
	var out struct {
		Data   *json.RawMessage
		Errors errors
		//Extensions interface{} // Unused.
	}
	err = json.NewDecoder(resp.Body).Decode(&out)
	if err != nil {
		// TODO: Consider including response body in returned error, if deemed helpful.
		return err
	}
	if out.Data != nil {
		err := jsonutil.UnmarshalGraphQL(*out.Data, v)
		if err != nil {
			// TODO: Consider including response body in returned error, if deemed helpful.
			return err
		}
	}
	if len(out.Errors) > 0 {
		return out.Errors
	}
	return nil
}

// errors represents the "errors" array in a response from a GraphQL server.
// If returned via error interface, the slice is expected to contain at least 1 element.
//
// Specification: https://facebook.github.io/graphql/#sec-Errors.
type errors []struct {
	Message   string
	Locations []struct {
		Line   int
		Column int
	}
}

// Error implements error interface.
func (e errors) Error() string {
	return e[0].Message
}

type operationType uint8

const (
	queryOperation operationType = iota
	mutationOperation
	subscriptionOperation
)
