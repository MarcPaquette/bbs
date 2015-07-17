package events

import (
	"encoding/base64"
	"errors"
	"fmt"
	"io"

	"github.com/cloudfoundry-incubator/bbs/models"
	"github.com/gogo/protobuf/proto"
	"github.com/vito/go-sse/sse"
)

var ErrUnrecognizedEventType = errors.New("unrecognized event type")

var ErrSourceClosed = errors.New("source closed")

type invalidPayloadError struct {
	payloadType string
	protoErr    error
}

func NewInvalidPayloadError(payloadType string, protoErr error) error {
	return invalidPayloadError{payloadType: payloadType, protoErr: protoErr}
}

func (e invalidPayloadError) Error() string {
	return fmt.Sprintf("invalid protobuf payload of type %s: %s", e.payloadType, e.protoErr.Error())
}

type rawEventSourceError struct {
	rawError error
}

func NewRawEventSourceError(rawError error) error {
	return rawEventSourceError{rawError: rawError}
}

func (e rawEventSourceError) Error() string {
	return fmt.Sprintf("raw event source error: %s", e.rawError.Error())
}

type closeError struct {
	err error
}

func NewCloseError(err error) error {
	return closeError{err: err}
}

func (e closeError) Error() string {
	return fmt.Sprintf("error closing raw source: %s", e.err.Error())
}

//go:generate counterfeiter -o eventfakes/fake_event_source.go . EventSource

// EventSource provides sequential access to a stream of events.
type EventSource interface {
	// Next reads the next event from the source. If the connection is lost, it
	// automatically reconnects.
	//
	// If the end of the stream is reached cleanly (which should actually never
	// happen), io.EOF is returned. If called after or during Close,
	// ErrSourceClosed is returned.
	Next() (models.Event, error)

	// Close releases the underlying response, interrupts any in-flight Next, and
	// prevents further calls to Next.
	Close() error
}

//go:generate counterfeiter -o eventfakes/fake_raw_event_source.go . RawEventSource

type RawEventSource interface {
	Next() (sse.Event, error)
	Close() error
}

type eventSource struct {
	rawEventSource RawEventSource
}

func NewEventSource(raw RawEventSource) EventSource {
	return &eventSource{
		rawEventSource: raw,
	}
}

func (e *eventSource) Next() (models.Event, error) {
	rawEvent, err := e.rawEventSource.Next()
	if err != nil {
		switch err {
		case io.EOF:
			return nil, err

		case sse.ErrSourceClosed:
			return nil, ErrSourceClosed

		default:
			return nil, NewRawEventSourceError(err)
		}
	}

	return parseRawEvent(rawEvent)
}

func (e *eventSource) Close() error {
	err := e.rawEventSource.Close()
	if err != nil {
		return NewCloseError(err)
	}

	return nil
}

func parseRawEvent(rawEvent sse.Event) (models.Event, error) {
	data, err := base64.StdEncoding.DecodeString(string(rawEvent.Data))
	if len(data) == 0 || err != nil {
		return nil, NewInvalidPayloadError(rawEvent.Name, err)
	}
	switch rawEvent.Name {
	case models.EventTypeDesiredLRPCreated:
		event := new(models.DesiredLRPCreatedEvent)
		err := proto.Unmarshal(data, event)
		if err != nil {
			return nil, NewInvalidPayloadError(rawEvent.Name, err)
		}

		return event, nil

	case models.EventTypeDesiredLRPChanged:
		event := new(models.DesiredLRPChangedEvent)
		err := proto.Unmarshal(data, event)
		if err != nil {
			return nil, NewInvalidPayloadError(rawEvent.Name, err)
		}

		return event, nil

	case models.EventTypeDesiredLRPRemoved:
		event := new(models.DesiredLRPRemovedEvent)
		err := proto.Unmarshal(data, event)
		if err != nil {
			return nil, NewInvalidPayloadError(rawEvent.Name, err)
		}

		return event, nil

	case models.EventTypeActualLRPCreated:
		event := new(models.ActualLRPCreatedEvent)
		err := proto.Unmarshal(data, event)
		if err != nil {
			return nil, NewInvalidPayloadError(rawEvent.Name, err)
		}

		return event, nil

	case models.EventTypeActualLRPChanged:
		event := new(models.ActualLRPChangedEvent)
		err := proto.Unmarshal(data, event)
		if err != nil {
			return nil, NewInvalidPayloadError(rawEvent.Name, err)
		}

		return event, nil

	case models.EventTypeActualLRPRemoved:
		event := new(models.ActualLRPRemovedEvent)
		err := proto.Unmarshal(data, event)
		if err != nil {
			return nil, NewInvalidPayloadError(rawEvent.Name, err)
		}

		return event, nil
	}

	return nil, ErrUnrecognizedEventType
}
