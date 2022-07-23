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

package krie

import (
	"fmt"
	"os"
	"time"

	manager "github.com/DataDog/ebpf-manager"
	"github.com/sirupsen/logrus"

	"github.com/Gui774ume/krie/pkg/krie/events"
)

// KRIE is the main KRIE structure
type KRIE struct {
	event        *events.Event
	handleEvent  func(data []byte) error
	timeResolver *events.TimeResolver
	outputFile   *os.File

	options        Options
	manager        *manager.Manager
	managerOptions manager.Options

	startTime time.Time
	numCPU    int
}

// NewKRIE creates a new KRIE instance
func NewKRIE(options Options) (*KRIE, error) {
	var err error

	if err = options.IsValid(); err != nil {
		return nil, err
	}

	e := &KRIE{
		event:       events.NewEvent(),
		options:     options,
		handleEvent: options.EventHandler,
	}
	if e.handleEvent == nil {
		e.handleEvent = e.defaultEventHandler
	}

	e.timeResolver, err = events.NewTimeResolver()
	if err != nil {
		return nil, err
	}

	e.numCPU, err = NumCPU()
	if err != nil {
		return nil, err
	}

	if len(options.Output) > 0 {
		e.outputFile, err = os.Create(options.Output)
		if err != nil {
			return nil, fmt.Errorf("couldn't create output file: %w", err)
		}

		_ = os.Chmod(options.Output, 0644)
	}
	return e, nil
}

// Start hooks on the requested symbols and begins tracing
func (e *KRIE) Start() error {
	if err := e.startManager(); err != nil {
		return err
	}
	return nil
}

// Stop shuts down KRIE
func (e *KRIE) Stop() error {
	if e.manager == nil {
		// nothing to stop, return
		return nil
	}

	if err := e.manager.Stop(manager.CleanAll); err != nil {
		logrus.Errorf("couldn't stop manager: %v", err)
	}

	if e.outputFile != nil {
		if err := e.outputFile.Close(); err != nil {
			logrus.Errorf("couldn't close output file: %v", err)
		}
	}
	return nil
}

func (e *KRIE) pushFilters() error {
	return nil
}

var eventZero events.Event

func (e *KRIE) zeroEvent() *events.Event {
	*e.event = eventZero
	return e.event
}

func (e *KRIE) defaultEventHandler(data []byte) error {
	event := e.zeroEvent()

	// unmarshall kernel event
	cursor, err := event.Kernel.UnmarshalBinary(data, e.timeResolver)
	if err != nil {
		return err
	}

	// unmarshall process context
	read, err := event.Process.UnmarshalBinary(data[cursor:])
	if err != nil {
		return err
	}
	cursor += read

	switch event.Kernel.Type {
	case events.InitModuleEventType:
		read, err = event.InitModule.UnmarshallBinary(data[cursor:])
		if err != nil {
			return err
		}
	case events.DeleteModuleEventType:
		read, err = event.DeleteModule.UnmarshallBinary(data[cursor:])
		if err != nil {
			return err
		}
	case events.BPFEventType:
		read, err = event.BPFEvent.UnmarshallBinary(data[cursor:])
		if err != nil {
			return err
		}
	case events.BPFFilterEventType:
		read, err = event.BPFFilterEvent.UnmarshallBinary(data[cursor:])
		if err != nil {
			return err
		}
	default:
		return fmt.Errorf("unknown event type: %s", event.Kernel.Type)
	}
	cursor += read

	// write to output file
	if e.outputFile != nil {
		var jsonData []byte
		jsonData, err = event.MarshalJSON()
		if err != nil {
			return fmt.Errorf("couldn't marshall event: %w", err)
		}
		jsonData = append(jsonData, "\n"...)
		if _, err = e.outputFile.Write(jsonData); err != nil {
			return fmt.Errorf("couldn't write event to output: %w", err)
		}
	}

	if logrus.GetLevel() >= logrus.DebugLevel {
		logrus.Debugf("%s", event.String())
	}
	return nil
}
