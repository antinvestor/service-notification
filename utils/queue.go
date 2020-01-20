package utils

import (
	"fmt"
	"github.com/nats-io/nats.go"
	"github.com/nats-io/stan.go"
	"github.com/sirupsen/logrus"
	"time"
)

// ConfigureQueue StanQueue Access for environment is configured here
func ConfigureQueue(log *logrus.Entry) (stan.Conn, *StanQueue, error) {

	queueURL := GetEnv("QUEUE_URL", nats.DefaultURL)

	// Connect to a server
	nc, err := nats.Connect(queueURL,
		nats.ReconnectBufSize(50*1024*1024), nats.ReconnectWait(1*time.Second),
		nats.DisconnectErrHandler(func(nc *nats.Conn, err error) {
			log.Infof("queue got disconnected! Reason: %q", err)
		}),
		nats.ReconnectHandler(func(nc *nats.Conn) {
			log.Infof("queue got reconnected to %v!\n", nc.ConnectedUrl())
		}),
		nats.ClosedHandler(func(nc *nats.Conn) {
			log.Infof("queue connection closed. Reason: %q\n", nc.LastError())
		}))

	if err != nil {
		return nil, nil,  err
	}

	stanQueue, err := NewQue(log, nc)
	if err != nil {
		return nil, nil,  err
	}

	return stanQueue.connection, stanQueue, nil
}


// StanQueue implements the "ICheckable" interface,
// this is our gateway to health checking
type StanQueue struct {
	connection stan.Conn
	log *logrus.Entry
	disconnected bool
}
//ConnectionLostListener func
func(q *StanQueue) ConnectionLostListener(conn stan.Conn, reason error) {
	q.log.Errorf("Connection lost, reason: %v", reason)
	q.disconnected = true
}

//NewQue mm
func NewQue(log *logrus.Entry, conn *nats.Conn) (*StanQueue, error) {

	clusterID := GetEnv("QUEUE_CLUSTER_ID", "test cluster")
	clientID := GetEnv("QUEUE_CLIENT_ID", fmt.Sprintf("notification_router", ))

	stanQueue := StanQueue{
		log: log,
		disconnected: false,
	}

	stanConnection, err := stan.Connect(clusterID, clientID, stan.NatsConn(conn),
		stan.Pings(10, 5),
		stan.SetConnectionLostHandler(stanQueue.ConnectionLostListener))

	if err != nil {
		return nil, err
	}

	stanQueue.connection = stanConnection
	return &stanQueue, nil
}

// this makes sure the queue check is properly configured
func validateQueConfig(conn stan.Conn) error {
	if conn == nil {
		return fmt.Errorf("A connection is required")
	}

	if conn.NatsConn() == nil  {
		return fmt.Errorf("Underlaying connection is missing")
	}

	if !conn.NatsConn().IsConnected()  {
		return fmt.Errorf("Available connection is not connected ")
	}

	return nil
}

// Status is used for performing a queue ping
// the "ICheckable" interface.
func (q *StanQueue) Status() (interface{}, error) {
	if err := validateQueConfig(q.connection); err != nil {
		return nil, err
	}

	if q.disconnected {
		return nil, fmt.Errorf("our queue connection was lost")
	}

	return nil, nil
}
