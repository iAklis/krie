/*
Copyright © 2022 GUILLAUME FOURNIER

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

//go:generate go run github.com/mailru/easyjson/easyjson -no_std_marshalers $GOFILE

package events

import (
	"fmt"
	"strings"

	manager "github.com/DataDog/ebpf-manager"
	"github.com/mailru/easyjson/jwriter"
)

const (
	// KRIEUID is the UID used to uniquely identify kernel space programs
	KRIEUID = "krie"
)

// EventType describes the type of an event sent from the kernel
type EventType uint32

const (
	// UnknownEventType unknow event
	UnknownEventType EventType = iota
	// InitModuleEventType is the event type of an init module event
	InitModuleEventType
	// DeleteModuleEventType is the event type of a delete module event
	DeleteModuleEventType
	// BPFEventType is the event type of a BPF event
	BPFEventType
	// BPFFilterEventType is the event type of a bpf_filter event
	BPFFilterEventType
	// PTraceEventType is the event type of a ptrace event
	PTraceEventType
	// KProbeEventType is the event type of a kprobe event
	KProbeEventType
	// SysCtlEventType  is the event type of a sysctl event
	SysCtlEventType
	// MaxEventType is used internally to get the maximum number of events.
	MaxEventType
)

func (t EventType) String() string {
	switch t {
	case InitModuleEventType:
		return "init_module"
	case DeleteModuleEventType:
		return "delete_module"
	case BPFEventType:
		return "bpf"
	case BPFFilterEventType:
		return "bpf_event"
	case PTraceEventType:
		return "ptrace"
	case KProbeEventType:
		return "kprobe"
	case SysCtlEventType:
		return "sysctl"
	default:
		return fmt.Sprintf("EventType(%d)", t)
	}
}

func (t EventType) MarshalJSON() ([]byte, error) {
	return []byte(fmt.Sprintf("\"%s\"", t.String())), nil
}

var eventTypeStrings = map[string]EventType{}

func init() {
	for i := EventType(0); i < MaxEventType; i++ {
		eventTypeStrings[i.String()] = i
	}
}

// ParseEventType returns an event type from its string representation
func ParseEventType(input string) EventType {
	return eventTypeStrings[input]
}

// EventTypeList is a list of EventType
type EventTypeList []EventType

func (etl EventTypeList) String() string {
	switch len(etl) {
	case 0:
		return ""
	case 1:
		return etl[0].String()
	}
	n := len(etl) - 1
	for i := 0; i < len(etl); i++ {
		n += len(etl[i].String())
	}

	var b strings.Builder
	b.Grow(n)
	b.WriteString(etl[0].String())
	for _, s := range etl[1:] {
		b.WriteString(",")
		b.WriteString(s.String())
	}
	return b.String()
}

// Insert inserts an event type in a list of event type
func (etl *EventTypeList) Insert(et EventType) {
	for _, elem := range *etl {
		if et == elem {
			return
		}
	}
	*etl = append(*etl, et)
}

// Contains return true if the list of event types is empty or if it contains the provided event type
func (etl *EventTypeList) Contains(et EventType) bool {
	if len(*etl) == 0 {
		return true
	}

	for _, elem := range *etl {
		if elem == et {
			return true
		}
	}
	return false
}

// UnmarshalYAML parses a string representation of a list of event types
func (etl *EventTypeList) UnmarshalYAML(unmarshal func(interface{}) error) error {
	var eventTypes []string
	err := unmarshal(&eventTypes)
	if err != nil {
		return fmt.Errorf("failed to unmarshal the list of event types: %w", err)
	}

	for _, et := range eventTypes {
		// check if the provided event type exists
		newEventType := ParseEventType(et)
		if newEventType == UnknownEventType {
			return fmt.Errorf("unknown event type: %s", et)
		}
		etl.Insert(newEventType)
	}
	return nil
}

// AllProbesSelectors returns all the probes selectors
func AllProbesSelectors(events EventTypeList) []manager.ProbesSelector {
	all := []manager.ProbesSelector{
		&manager.AllOf{
			Selectors: []manager.ProbesSelector{
				&manager.ProbeSelector{ProbeIdentificationPair: manager.ProbeIdentificationPair{UID: KRIEUID, EBPFSection: "tracepoint/raw_syscalls/sys_exit", EBPFFuncName: "sys_exit"}},
			},
		},
	}
	addAllKernelModuleProbesSelectors(&all, events)
	if events.Contains(BPFEventType) {
		addBPFProbesSelectors(&all)
	}
	if events.Contains(BPFFilterEventType) {
		addSetSockOptSelectors(&all)
	}
	if events.Contains(PTraceEventType) {
		addPTraceSelectors(&all)
	}
	if events.Contains(KProbeEventType) {
		addKProbeSelectors(&all)
	}
	if events.Contains(SysCtlEventType) {
		addSysCtlSelectors(&all)
	}
	return all
}

// AllProbes returns all the probes
func AllProbes(events EventTypeList) []*manager.Probe {
	all := []*manager.Probe{
		{
			ProbeIdentificationPair: manager.ProbeIdentificationPair{
				UID:          KRIEUID,
				EBPFSection:  "tracepoint/raw_syscalls/sys_exit",
				EBPFFuncName: "sys_exit",
			},
		},
	}
	addKernelModuleProbes(&all, events)
	if events.Contains(BPFEventType) {
		addBPFProbes(&all)
	}
	if events.Contains(BPFFilterEventType) {
		addSetSockOptProbes(&all)
	}
	if events.Contains(PTraceEventType) {
		addPTraceProbes(&all)
	}
	if events.Contains(KProbeEventType) {
		addKProbeProbes(&all)
	}
	if events.Contains(SysCtlEventType) {
		addSysCtlProbes(&all)
	}

	return all
}

// AllTailCallRoutes returns all the tail call routes
func AllTailCallRoutes(events EventTypeList) []manager.TailCallRoute {
	var all []manager.TailCallRoute

	addKernelModuleTailCallRoutes(&all, events)
	if events.Contains(BPFEventType) {
		addBPFTailCallRoutes(&all)
	}
	if events.Contains(BPFFilterEventType) {
		addSetSockOptRoutes(&all)
	}
	if events.Contains(PTraceEventType) {
		addPTraceRoutes(&all)
	}
	if events.Contains(KProbeEventType) {
		addKProbeRoutes(&all)
	}
	if events.Contains(SysCtlEventType) {
		addSysCtlRoutes(&all)
	}
	return all
}

// Event is used to parse the events sent from kernel space
type Event struct {
	Kernel  KernelEvent
	Process ProcessContext

	InitModule     InitModuleEvent
	DeleteModule   DeleteModuleEvent
	BPFEvent       BPFEvent
	BPFFilterEvent BPFFilterEvent
	PTraceEvent    PTraceEvent
	KProbeEvent    KProbeEvent
	SysCtlEvent    SysCtlEvent
}

// NewEvent returns a new Event instance
func NewEvent() *Event {
	return &Event{}
}

func (e *Event) MarshalJSON() ([]byte, error) {
	s := NewEventSerializer(e)
	w := &jwriter.Writer{
		Flags: jwriter.NilSliceAsEmpty | jwriter.NilMapAsEmpty,
	}
	s.MarshalEasyJSON(w)
	return w.BuildBytes()
}

func (e Event) String() string {
	data, err := e.MarshalJSON()
	if err != nil {
		return fmt.Sprintf("failed to marshall event: %v", err)
	}
	return string(data)
}

// EventSerializer is used to serialize Event
// easyjson:json
type EventSerializer struct {
	*KernelEventSerializer    `json:"event,omitempty"`
	*ProcessContextSerializer `json:"process,omitempty"`

	*InitModuleEventSerializer   `json:"init_module,omitempty"`
	*DeleteModuleEventSerializer `json:"delete_module,omitempty"`
	*BPFEventSerializer          `json:"bpf,omitempty"`
	*BPFFilterEventSerializer    `json:"bpf_filter,omitempty"`
	*PtraceEventSerializer       `json:"ptrace,omitempty"`
	*KProbeEventSerializer       `json:"kprobe,omitempty"`
	*SysCtlEventEventSerializer  `json:"sysctl,omitempty""`
}

// NewEventSerializer returns a new EventSerializer instance for the provided Event
func NewEventSerializer(event *Event) *EventSerializer {
	serializer := &EventSerializer{
		KernelEventSerializer:    NewKernelEventSerializer(&event.Kernel),
		ProcessContextSerializer: NewProcessContextSerializer(&event.Process),
	}

	switch event.Kernel.Type {
	case InitModuleEventType:
		serializer.InitModuleEventSerializer = NewInitModuleSerializer(&event.InitModule)
	case DeleteModuleEventType:
		serializer.DeleteModuleEventSerializer = NewDeleteModuleSerializer(&event.DeleteModule)
	case BPFEventType:
		serializer.BPFEventSerializer = NewBPFEventSerializer(&event.BPFEvent)
	case BPFFilterEventType:
		serializer.BPFFilterEventSerializer = NewBPFFilterEventSerializer(&event.BPFFilterEvent)
	case PTraceEventType:
		serializer.PtraceEventSerializer = NewPtraceEventSerializer(&event.PTraceEvent)
	case KProbeEventType:
		serializer.KProbeEventSerializer = NewKProbeEventSerializer(&event.KProbeEvent)
	case SysCtlEventType:
		serializer.SysCtlEventEventSerializer = NewSysCtlEventSerializer(&event.SysCtlEvent)
	}
	return serializer
}
