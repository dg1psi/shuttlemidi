package devices

import (
	"errors"

	"github.com/bearsh/hid"
)

// USB HID device information
const (
	shuttlexpress_vendorId  = 0x0b33
	shuttlexpress_productId = 0x0020
)

var (
	ErrShuttleExpressDeviceNotFound  = errors.New("no ShuttlExpress found")
	ErrShuttleExpressDeviceNotOpened = errors.New("ShuttlExpress: No device opened")
)

// ShuttleStatus contains a event channel for all ShuttlExpress hardware controls.
// The channels have to be created by the consuming module.
type ShuttleStatus struct {
	Wheel_position  chan int8
	Dial_direction  chan int8
	Button1_pressed chan bool
	Button2_pressed chan bool
	Button3_pressed chan bool
	Button4_pressed chan bool
	Button5_pressed chan bool

	wheel_value   int8
	dial_value    uint8
	button1_value bool
	button2_value bool
	button3_value bool
	button4_value bool
	button5_value bool
}

// ShuttlExpress Driver based on the hardware information from the Python implementation https://github.com/EMATech/ContourShuttleXpress
type ShuttlExpress struct {
	devhandle *hid.Device
	devinfo   hid.DeviceInfo
	err       error

	ShuttleStatus
}

// readdevice is a goroutine and continously reads the device status and sends out events through the channels part of ShuttleStatus
func (se *ShuttlExpress) readdevice() {
	if se.devhandle == nil {
		se.err = ErrShuttleExpressDeviceNotOpened
		return
	}
	se.devhandle.SetNonblocking(false)

	for {
		var buf = make([]byte, 48)
		if _, err := se.devhandle.Read(buf); err != nil {
			se.err = err
			return
		}
		wheel_pos := int8(buf[0])
		dial_pos := uint8(buf[1])
		b1_pressed := buf[3]&(1<<4) > 0
		b2_pressed := buf[3]&(1<<5) > 0
		b3_pressed := buf[3]&(1<<6) > 0
		b4_pressed := buf[3]&(1<<7) > 0
		b5_pressed := buf[4]&(1<<0) > 0

		if wheel_pos != se.wheel_value && se.Wheel_position != nil {
			se.Wheel_position <- wheel_pos
			se.wheel_value = wheel_pos
		}
		if dial_pos != se.dial_value && se.Dial_direction != nil {
			dial_delta := int8(dial_pos - se.dial_value)

			// only use if difference is a single step. Else it's the first read
			if dial_delta == 1 || dial_delta == -1 {
				se.Dial_direction <- dial_delta
			}
			se.dial_value = dial_pos
		}
		if b1_pressed != se.button1_value && se.Button1_pressed != nil {
			se.Button1_pressed <- b1_pressed
			se.button1_value = b1_pressed
		}
		if b2_pressed != se.button2_value && se.Button1_pressed != nil {
			se.Button2_pressed <- b2_pressed
			se.button2_value = b2_pressed
		}
		if b3_pressed != se.button3_value && se.Button1_pressed != nil {
			se.Button3_pressed <- b3_pressed
			se.button3_value = b3_pressed
		}
		if b4_pressed != se.button4_value && se.Button1_pressed != nil {
			se.Button4_pressed <- b4_pressed
			se.button4_value = b4_pressed
		}
		if b5_pressed != se.button5_value && se.Button1_pressed != nil {
			se.Button5_pressed <- b5_pressed
			se.button5_value = b5_pressed
		}
	}
}

// NewShuttlExpress searches for available ShuttlExpress devices and opens the first one it finds
func NewShuttlExpress() (*ShuttlExpress, error) {

	di := hid.Enumerate(shuttlexpress_vendorId, shuttlexpress_productId)
	if len(di) == 0 {
		return nil, ErrShuttleExpressDeviceNotFound
	}

	dev, err := di[0].Open()
	if err != nil {
		return nil, err
	}

	status := ShuttleStatus{}
	se := &ShuttlExpress{devhandle: dev, devinfo: di[0], err: nil, ShuttleStatus: status}
	go se.readdevice()

	return se, nil
}
