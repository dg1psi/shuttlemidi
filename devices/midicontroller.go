package devices

import (
	"errors"
	"log"
	"strings"
	"time"

	"gitlab.com/gomidi/midi"
	"gitlab.com/gomidi/midi/writer"
	"gitlab.com/gomidi/rtmididrv"
)

const (
	midiMaxRepeat = 50 // maximum number a message is repeated
)

var (
	ErrMIDIDeviceNotFound       = errors.New("MIDI Device not found")
	ErrMIDIDeviceNotInitialized = errors.New("MIDI Device not initialized")
)

// MidiController is the public interface to send out MIDI controller messages to a device
type MidiController interface {
	Open() error
	Close() error
	SendCommand(controller uint8, value uint8, repeat bool) error
}

// midiControllerCommand contains a single command that will be send out
type midiControllerCommand struct {
	controller uint8
	value      uint8
	repeat     bool
}

// midiControl contains all driver and channel variables in required for the communication
type midiControl struct {
	DeviceName string
	Delay      time.Duration
	Channel    uint8
	drv        midi.Driver
	output     midi.Out
	wr         *writer.Writer

	commandch chan *midiControllerCommand
	quitch    chan struct{}
}

// commandExecutor sends out MIDI messages received through the commandch channel. It also takes care of sending messages out
// repeatedly, in case it is requested
func (mc *midiControl) commandExecutor() {
	type tickstruct struct {
		counter int
		value   uint8
	}
	repeatcmd := make(map[uint8]tickstruct)
	tick := time.NewTicker(mc.Delay)
	defer tick.Stop()

	tick.Stop()

	for {
		select {
		case <-mc.quitch:
			return
		case cmd := <-mc.commandch:
			log.Printf("Controller: %v, Value: %v, Repeat: %v\n", cmd.controller, cmd.value, cmd.repeat)
			writer.ControlChange(mc.wr, cmd.controller, cmd.value)
			if cmd.repeat {
				repeatcmd[cmd.controller] = tickstruct{counter: midiMaxRepeat, value: cmd.value}
				tick.Reset(mc.Delay)
			} else {
				delete(repeatcmd, cmd.controller)
				if len(repeatcmd) == 0 {
					tick.Stop()
				}
			}
		case <-tick.C:
			for k, v := range repeatcmd {
				if v.counter > 1 {
					log.Printf("Controller: %v, Value: %v, Repeat-Counter: %v\n", k, v.value, v.counter)
					writer.ControlChange(mc.wr, k, v.value)
					v.counter--
					repeatcmd[k] = v
				} else {
					delete(repeatcmd, k)
				}
			}
			if len(repeatcmd) == 0 {
				tick.Stop()
			}
		}
	}
}

// Open connects to the driver specified during instance creation, sets the channel used for the MIDI messages and starts
// the goroutine used for message sending
func (mc *midiControl) Open() error {

	if mc.drv == nil {
		drv, err := rtmididrv.New()
		if err != nil {
			log.Println(err)
		}
		mc.drv = drv
	}

	outs, err := mc.drv.Outs()
	if err != nil {
		log.Println(err)
	}
	for i, v := range outs {
		log.Printf("%v: %v\n", i, v.String())
		if strings.Contains(v.String(), mc.DeviceName) {
			mc.output = outs[i]
		}
	}

	if mc.output == nil {
		return ErrMIDIDeviceNotFound
	}

	if err := mc.output.Open(); err != nil {
		log.Println(err)
	}

	mc.wr = writer.New(mc.output)
	mc.wr.SetChannel(mc.Channel)

	mc.commandch = make(chan *midiControllerCommand, 1)
	mc.quitch = make(chan struct{})

	go mc.commandExecutor()
	return nil
}

// SendCommand sends a ControllerChange MIDI command to the current MIDI device. If repeat is true then the message
// will be send up to midiMaxRepeat times with a delay as specified during instance creation
func (mc *midiControl) SendCommand(controller uint8, value uint8, repeat bool) error {
	if mc.output == nil {
		return ErrMIDIDeviceNotInitialized
	}
	cmd := &midiControllerCommand{controller: controller, value: value, repeat: repeat}
	mc.commandch <- cmd

	return nil
}

// Close stops the goroutine and closes all channels and drivers
func (mc *midiControl) Close() error {
	close(mc.quitch)

	errout := mc.output.Close()
	errdrv := mc.drv.Close()

	if errout != nil {
		return errout
	} else if errdrv != nil {
		return errdrv
	}
	return nil
}

// NewMIDIController creates a new MidiController instance with the specified parameters. If nil is passed as driver
// the default driver will be used (rtmididrv).
// delay specifies the time between each command message, in case the message should be send repeatedly.
func NewMIDIController(driver midi.Driver, devicename string, delay time.Duration, channel uint8) MidiController {
	return &midiControl{drv: driver, DeviceName: devicename, Delay: delay, Channel: channel}
}

// GetMIDIDevices returns a list of all devices availalbe for the specified driver. If nil is passed as driver
// the default driver will be used (rtmididrv)
func GetMIDIDevices(driver midi.Driver) ([]string, error) {
	var drv midi.Driver
	var err error

	if driver == nil {
		drv, err = rtmididrv.New()
		if err != nil {
			return nil, err
		}
		defer drv.Close()
	} else {
		drv = driver
	}

	outs, err := drv.Outs()
	if err != nil {
		return nil, err
	}

	result := make([]string, 0, len(outs))
	for _, v := range outs {
		result = append(result, v.String())
	}
	return result, nil
}
