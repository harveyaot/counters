// Package counters provides a simple counter, max and min functionalities.
// All counters are kept in CounterBox.
// Library is thread safe.
package counters

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"sync"
	"sync/atomic"
	"text/template"
)

// MaxMinValue is an interface for minima and maxima counters.
type MaxMinValue interface {
	// Set allows to update value if necessary.
	Set(int)
	// Name returns a name of counter.
	Name() string
	// Value returns a current value.
	Value() int64
}

// Counter is an interface for integer increase only counter.
type Counter interface {
	// Increment increases counter by one.
	Increment()
	// IncrementBy increases counter by given number.
	IncrementBy(num int)
	// Name returns a name of counter.
	Name() string
	// Value returns a current value of counter.
	Value() int64
}

// CounterBox is a main type, it keeps references to all counters
// requested from it.
type CounterBox struct {
	counters map[string]*counterImpl
	min      map[string]*minImpl
	max      map[string]*maxImpl
	m        *sync.RWMutex
}

// NewCounterBox creates a new object to keep all counters.
func NewCounterBox() *CounterBox {
	return &CounterBox{
		counters: make(map[string]*counterImpl),
		min:      make(map[string]*minImpl),
		max:      make(map[string]*maxImpl),
		m:        &sync.RWMutex{},
	}
}

// CreateHttpHandler creates a simple handler printing values of all counters.
func (c *CounterBox) CreateHttpHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		c.m.RLock()
		defer c.m.RUnlock()
		fmt.Fprintf(w, "Counters %d\n", len(c.counters))
		for k, v := range c.counters {
			fmt.Fprintf(w, "%s=%d\n", k, v.Value())
		}
		fmt.Fprintf(w, "\nMax values %d\n", len(c.max))
		for k, v := range c.max {
			fmt.Fprintf(w, "%s=%d\n", k, v.Value())
		}
		fmt.Fprintf(w, "\nMin values %d\n", len(c.min))
		for k, v := range c.min {
			fmt.Fprintf(w, "%s=%d\n", k, v.Value())
		}
	}
}

// GetCounter returns a counter of given name, if doesn't exist than create.
func (c *CounterBox) GetCounter(name string) Counter {
	c.m.RLock()
	if v, ok := c.counters[name]; ok {
		c.m.RUnlock()
		return v
	}
	c.m.RUnlock()
	c.m.Lock()
	defer c.m.Unlock()

	v := &counterImpl{name, 0}
	c.counters[name] = v
	return v
}

// GetMin returns a minima counter of given name, if doesn't exist than create.
func (c *CounterBox) GetMin(name string) MaxMinValue {
	c.m.RLock()
	if v, ok := c.min[name]; ok {
		c.m.RUnlock()
		return v
	}
	c.m.RUnlock()
	c.m.Lock()
	defer c.m.Unlock()

	v := &minImpl{name, 0}
	c.min[name] = v
	return v
}

// GetMax returns a maxima counter of given name, if doesn't exist than create.
func (c *CounterBox) GetMax(name string) MaxMinValue {
	c.m.RLock()
	if v, ok := c.max[name]; ok {
		c.m.RUnlock()
		return v
	}
	c.m.RUnlock()
	c.m.Lock()
	defer c.m.Unlock()

	v := &maxImpl{name, 0}
	c.max[name] = v
	return v
}

var tmpl = template.Must(template.New("main").Parse(`== Counters ==
{{- range .Counters}}
  {{.Name}}: {{.Value}}
{{- end}}
== Min values ==
{{- range .Min}}
  {{.Name}}: {{.Value}}
{{- end}}
== Max values ==
{{- range .Max}}
  {{.Name}}: {{.Value}}
{{- end -}}
`))

func (c *CounterBox) WriteTo(w io.Writer) {
	c.m.RLock()
	defer c.m.RUnlock()
	data := &struct {
		Counters []Counter
		Min      []MaxMinValue
		Max      []MaxMinValue
	}{}
	for _, c := range c.counters {
		data.Counters = append(data.Counters, c)
	}
	for _, c := range c.min {
		data.Min = append(data.Min, c)
	}
	for _, c := range c.max {
		data.Max = append(data.Max, c)
	}
	tmpl.Execute(w, data)
}

func (c *CounterBox) String() string {
	buf := &bytes.Buffer{}
	c.WriteTo(buf)
	return buf.String()
}

type counterImpl struct {
	name  string
	value int64
}

func (c *counterImpl) Increment() {
	atomic.AddInt64(&c.value, 1)
}

func (c *counterImpl) IncrementBy(num int) {
	atomic.AddInt64(&c.value, int64(num))
}

func (c *counterImpl) Name() string {
	return c.name
}

func (c *counterImpl) Value() int64 {
	return atomic.LoadInt64(&c.value)
}

type maxImpl counterImpl

func (m *maxImpl) Set(v int) {
	done := false
	v64 := int64(v)
	for !done {
		if o := atomic.LoadInt64(&m.value); v64 > o {
			done = atomic.CompareAndSwapInt64(&m.value, o, v64)
		} else {
			done = true
		}
	}
}

func (m *maxImpl) Name() string {
	return m.name
}

func (m *maxImpl) Value() int64 {
	return atomic.LoadInt64(&m.value)
}

type minImpl counterImpl

func (m *minImpl) Set(v int) {
	done := false
	v64 := int64(v)
	for !done {
		if o := atomic.LoadInt64(&m.value); v64 < o {
			done = atomic.CompareAndSwapInt64(&m.value, o, v64)
		} else {
			done = true
		}
	}
}

func (m *minImpl) Name() string {
	return m.name
}

func (m *minImpl) Value() int64 {
	return atomic.LoadInt64(&m.value)
}