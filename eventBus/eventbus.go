package eventbus

import (
	"fmt"
	"reflect"
	"sync"
	"time"
)

// BusSubscriber defines subscription-related bus behavior
type BusSubscriber interface {
	Subscribe(topic string, fn interface{}) error
	Unsubscribe(topic string, handler interface{}) error
}

// BusPublisher defines publishing-related bus behavior
type BusPublisher interface {
	Publish(topic string, args ...interface{})
	PublishWithReply(topic string, timeout time.Duration, args ...interface{}) (interface{}, error) // 目前只支持返回一个函数的handler
}

// BusController defines bus control behavior (checking handler's presence, synchronization)
type BusController interface {
	HasCallback(topic string) bool
}

// Bus englobes global (subscribe, publish, control) bus behavior
type Bus interface {
	BusController
	BusSubscriber
	BusPublisher
}

// EventBus - box for handlers and callbacks.
type EventBus struct {
	handlers map[string][]*eventHandler
	lock     sync.RWMutex // a rwlock for the map
}

type eventHandler struct {
	callBack   reflect.Value
	sync.Mutex // lock for an event handler - useful for running async callbacks serially
}

// New returns new EventBus with empty handlers.
func New() Bus {
	b := &EventBus{
		make(map[string][]*eventHandler),
		sync.RWMutex{},
	}
	return Bus(b)
}

// doSubscribe handles the subscription logic and is utilized by the public Subscribe functions
func (bus *EventBus) doSubscribe(topic string, fn interface{}, handler *eventHandler) error {
	bus.lock.Lock()
	defer bus.lock.Unlock()
	if !(reflect.TypeOf(fn).Kind() == reflect.Func) {
		return fmt.Errorf("%s is not of type reflect.Func", reflect.TypeOf(fn).Kind())
	}

	bus.handlers[topic] = append(bus.handlers[topic], handler)
	return nil
}

// Subscribe subscribes to a topic.
// Returns error if `fn` is not a function.
func (bus *EventBus) Subscribe(topic string, fn interface{}) error {

	return bus.doSubscribe(topic, fn, &eventHandler{
		reflect.ValueOf(fn), sync.Mutex{},
	})
}

// HasCallback returns true if exists any callback subscribed to the topic.
func (bus *EventBus) HasCallback(topic string) bool {
	bus.lock.Lock()
	defer bus.lock.Unlock()
	_, ok := bus.handlers[topic]
	if ok {
		return len(bus.handlers[topic]) > 0
	}
	return false
}

// Unsubscribe removes callback defined for a topic.
// Returns error if there are no callbacks subscribed to the topic.
func (bus *EventBus) Unsubscribe(topic string, handler interface{}) error {
	bus.lock.Lock()
	defer bus.lock.Unlock()
	if _, ok := bus.handlers[topic]; ok && len(bus.handlers[topic]) > 0 {
		bus.removeHandler(topic, bus.findHandlerIdx(topic, reflect.ValueOf(handler)))
		return nil
	}
	return fmt.Errorf("topic %s doesn't exist", topic)
}

// Publish executes callback defined for a topic. Any additional argument will be transferred to the callback.
func (bus *EventBus) Publish(topic string, args ...interface{}) {
	bus.lock.RLock() // will unlock if handler is not found or always after setUpPublish
	defer bus.lock.RUnlock()
	if handlers, ok := bus.handlers[topic]; ok && 0 < len(handlers) {
		// Handlers slice may be changed by removeHandler and Unsubscribe during iteration,
		// so make a copy and iterate the copied slice.
		copyHandlers := make([]*eventHandler, len(handlers))
		copy(copyHandlers, handlers)
		for _, handler := range copyHandlers {
			go bus.doPublish(handler, topic, args...)
		}
	}
}

// 向一个主题推送消息，支持单handler的主题，并返回这个handler的值
func (bus *EventBus) PublishWithReply(topic string, timeout time.Duration, args ...interface{}) (interface{}, error) {
	bus.lock.RLock()
	defer bus.lock.RUnlock()

	// 如果对应的topic有多个则返回error
	handlers, ok := bus.handlers[topic]
	if !ok {
		return nil, fmt.Errorf("topic(%s)没有注册", topic)
	}
	handlerNum := len(handlers)
	if handlerNum != 1 {
		return nil, fmt.Errorf("topic(%s)对应的handler个数错误[期望值: 1,实际值: %d]", topic, handlerNum)
	}
	copyHandlers := make([]*eventHandler, len(handlers))
	copy(copyHandlers, handlers)

	done := make(chan []reflect.Value, 1)
	defer close(done)
	go func() {
		defer func() {
			if err := recover(); err != nil {
				fmt.Printf("PublishWithReply painc (%s)\n", err)
			}
		}()
		passedArguments := bus.setUpPublish(copyHandlers[0], args...)
		resultList := copyHandlers[0].callBack.Call(passedArguments)
		done <- resultList
	}()

	select {
	case resultList := <-done:
		if len(resultList) == 1 {
			return resultList[0], nil
		}
		return nil, nil
	case <-time.After(timeout): // 超时返回
		return nil, fmt.Errorf("topic(%s)对应的handler执行超时", topic)
	}
}

func (bus *EventBus) doPublish(handler *eventHandler, topic string, args ...interface{}) {
	passedArguments := bus.setUpPublish(handler, args...)
	handler.callBack.Call(passedArguments)
}

func (bus *EventBus) removeHandler(topic string, idx int) {
	if _, ok := bus.handlers[topic]; !ok {
		return
	}
	l := len(bus.handlers[topic])

	if !(0 <= idx && idx < l) {
		return
	}

	copy(bus.handlers[topic][idx:], bus.handlers[topic][idx+1:])
	bus.handlers[topic][l-1] = nil // or the zero value of T
	bus.handlers[topic] = bus.handlers[topic][:l-1]
}

func (bus *EventBus) findHandlerIdx(topic string, callback reflect.Value) int {
	if _, ok := bus.handlers[topic]; ok {
		for idx, handler := range bus.handlers[topic] {
			if handler.callBack.Type() == callback.Type() &&
				handler.callBack.Pointer() == callback.Pointer() {
				return idx
			}
		}
	}
	return -1
}

func (bus *EventBus) setUpPublish(callback *eventHandler, args ...interface{}) []reflect.Value {
	funcType := callback.callBack.Type()
	passedArguments := make([]reflect.Value, len(args))
	for i, v := range args {
		if v == nil {
			passedArguments[i] = reflect.New(funcType.In(i)).Elem()
		} else {
			passedArguments[i] = reflect.ValueOf(v)
		}
	}

	return passedArguments
}
