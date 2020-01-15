package utils

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/nats-io/nats.go"
	"github.com/nats-io/stan.go"
	"github.com/opentracing/opentracing-go"
	"github.com/sirupsen/logrus"
	"strings"
	"time"
)

func QueueMakeID(ctx context.Context, id string) ([]byte, error) {

	queueMap := make(map[string]string)
	queueMap["q_id"] = id

	//Serialize span
	if span := opentracing.SpanFromContext(ctx); span != nil {

		carrier := opentracing.TextMapCarrier(queueMap)
		err := opentracing.GlobalTracer().Inject(
			span.Context(),
			opentracing.TextMap,
			carrier)
		if err != nil {
			return nil, err
		}
	}

	return json.Marshal(queueMap)
}

func QueueGetID(msg *stan.Msg, traceId string) (string, context.Context, opentracing.Span, error) {
	var queueMap map[string]string
	err := json.Unmarshal(msg.Data, &queueMap)
	if err != nil {
		return "", nil, nil, err
	}

	carrier := opentracing.TextMapCarrier(queueMap)
	spanCtx, err := opentracing.GlobalTracer().Extract(opentracing.TextMap, carrier)
	if err != nil {
		return "", nil, nil, err
	}

	bCtx := context.Background()
	span, ctx := opentracing.StartSpanFromContext(bCtx, traceId, opentracing.ChildOf(spanCtx))

	return queueMap["q_id"], ctx, span, nil
}

// ConfigureQueue StanQueue Access for environment is configured here
func ConfigureQueue(log *logrus.Entry) (stan.Conn, *StanQueue, error) {

	queueURL := GetEnv(EnvQueueUrl, nats.DefaultURL)

	// Connect to a server
	nc, err := nats.Connect(queueURL,
		nats.ReconnectBufSize(50*1024*1024), nats.ReconnectWait(1*time.Second),
		nats.DisconnectErrHandler(func(nc *nats.Conn, err error) {
			log.Infof("subscriptions got disconnected! Reason: %q", err)
		}),
		nats.ReconnectHandler(func(nc *nats.Conn) {
			log.Infof("subscriptions got reconnected to %v!\n", nc.ConnectedUrl())
		}),
		nats.ClosedHandler(func(nc *nats.Conn) {
			log.Infof("subscriptions connection closed. Reason: %q\n", nc.LastError())
		}))

	if err != nil {
		return nil, nil, err
	}

	stanQueue, err := newQue(log, nc)
	if err != nil {
		return nil, nil, err
	}

	return stanQueue.connection, stanQueue, nil
}

// StanQueue implements the "ICheckable" interface,
// this is our gateway to health checking
type StanQueue struct {
	connection   stan.Conn
	log          *logrus.Entry
	disconnected bool
}

func (q *StanQueue) ConnectionLostListener(conn stan.Conn, reason error) {
	q.log.Errorf("Connection lost, reason: %v", reason)
	q.disconnected = true
}

// Status is used for performing a subscriptions ping
// the "ICheckable" interface.
func (q *StanQueue) Status() (interface{}, error) {
	if err := validateQueConfig(q.connection); err != nil {
		return nil, err
	}

	if q.disconnected {
		return nil, fmt.Errorf("our subscriptions connection was lost")
	}

	return nil, nil
}

// this makes sure the subscriptions check is properly configured
func validateQueConfig(conn stan.Conn) error {
	if conn == nil {
		return fmt.Errorf("A connection is required")
	}

	if conn.NatsConn() == nil {
		return fmt.Errorf("Underlaying connection is missing")
	}

	if !conn.NatsConn().IsConnected() {
		return fmt.Errorf("Available connection is not connected ")
	}

	return nil
}

func newQue(log *logrus.Entry, conn *nats.Conn) (*StanQueue, error) {

	clusterID := GetEnv(EnvQueueClusterId, "ant")
	clientID := GetEnv(EnvQueueClientId, strings.Replace(GetMacAddress(), ":", "", -1))

	stanQueue := StanQueue{
		log:          log,
		disconnected: false,
	}

	stanConnection, err := stan.Connect(
		clusterID,
		clientID,
		stan.NatsConn(conn),
		stan.Pings(10, 5),
		stan.SetConnectionLostHandler(stanQueue.ConnectionLostListener))

	if err != nil {
		return nil, err
	}

	stanQueue.connection = stanConnection
	return &stanQueue, nil
}
