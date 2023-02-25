// ShuttleMidi sends MIDI events for the Contour ShuttlExpress to enable usage together with the MIDI Controller feature
// of [SDR Console](https://www.sdr-radio.com/Console).
//
// ShuttleMIDI doesn't provide it's own MIDI driver for Windows. It relies on the excellent and free
// loopMIDI driver from Tobias Erichsen: https://www.tobias-erichsen.de/software/loopmidi.html.
package main

import (
	"fmt"
	"strings"
	"time"

	"github.com/dg1psi/shuttlemidi/devices"
	icon "github.com/dg1psi/shuttlemidi/icons"
	"github.com/gen2brain/dlgs"
	"github.com/getlantern/systray"
	"github.com/spf13/viper"
)

const applicationName = "ShuttleMidi v0.1.3"

var (
	// configDefaults contain the default configuration written to the configuration file
	configDefaults = map[string]interface{}{
		"MidiDevice": "ShuttleMIDI",
	}
)

var mcontrol devices.MidiController

// quitch is the channel used to stop the goroutine handling the ShuttlExpress events
var quitch chan struct{}

// readshuttle is the goroutine used to handle all ShuttlExpress events and to send out the MIDI messages.
// The routine is stopped by closing the quitch channel
func readshuttle(quitch chan struct{}, se *devices.ShuttlExpress, mc devices.MidiController) {
	se.Wheel_position = make(chan int8)
	se.Dial_direction = make(chan int8)
	se.Button1_pressed = make(chan bool)
	se.Button2_pressed = make(chan bool)
	se.Button3_pressed = make(chan bool)
	se.Button4_pressed = make(chan bool)
	se.Button5_pressed = make(chan bool)

	for {
		select {
		case <-quitch:
			return
		case wp := <-se.Wheel_position:
			if wp > 0 && wp <= 7 {
				// Invert positive wheel positions to work around bug in SDR Console with Tune Up
				mc.SendCommand(0, uint8(18*(8-wp)), true)
			} else if wp >= -7 && wp < 0 {
				mc.SendCommand(1, uint8(18*(-wp)), true)
			} else {
				mc.SendCommand(0, 255, false)
				mc.SendCommand(1, 255, false)
			}
		case dd := <-se.Dial_direction:
			if dd == 1 {
				mc.SendCommand(2, 2, false)
			} else {
				mc.SendCommand(2, 1, false)
			}
		case b1 := <-se.Button1_pressed:
			if b1 {
				mc.SendCommand(3, 127, false)
			} else {
				mc.SendCommand(3, 0, false)
			}
		case b2 := <-se.Button2_pressed:
			if b2 {
				mc.SendCommand(4, 127, false)
			} else {
				mc.SendCommand(4, 0, false)
			}
		case b3 := <-se.Button3_pressed:
			if b3 {
				mc.SendCommand(5, 127, false)
			} else {
				mc.SendCommand(5, 0, false)
			}
		case b4 := <-se.Button4_pressed:
			if b4 {
				mc.SendCommand(6, 127, false)
			} else {
				mc.SendCommand(6, 0, false)
			}
		case b5 := <-se.Button5_pressed:
			if b5 {
				mc.SendCommand(7, 127, false)
			} else {
				mc.SendCommand(7, 0, false)
			}
		}
	}
}

// initSettings initializes the settings engine Viper. If it doesn't exist it is automatically created using the defaults
func initSettings() error {
	for k, v := range configDefaults {
		viper.SetDefault(k, v)
	}

	viper.SetConfigName("config")
	viper.SetConfigType("yaml")
	viper.AddConfigPath(".")
	if err := viper.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); ok {
			if err = viper.SafeWriteConfig(); err != nil {
				fmt.Println(err)
			}
		} else {
			fmt.Println(err)
			return err
		}
	}
	return nil
}

// startListeners creates and opens the specified MIDI device and starts the event handling goroutine readshuttle.
// In case the goroutine is already running it is restarted.
func startListeners(midiname string, se *devices.ShuttlExpress) {
	if quitch != nil {
		close(quitch)
		mcontrol.Close()
	}
	quitch = make(chan struct{})

	mcontrol = devices.NewMIDIController(nil, midiname, 100*time.Millisecond, 0)
	if err := mcontrol.Open(); err != nil {
		dlgs.Error(applicationName, "Unable to open MIDI device. Please select the correct device in the context menu.\n"+err.Error())
	} else {
		go readshuttle(quitch, se, mcontrol)
	}
}

// onReady is called by systray once the system tray menu can be created. It inializes the menu and opens the ShuttlExpress device
func onReady() {
	se, err := devices.NewShuttlExpress()
	if err != nil {
		if err == devices.ErrShuttleExpressDeviceNotFound {
			dlgs.Error(applicationName, "No ShuttlExpress device connected to this computer. Cannot continue.")
		} else {
			dlgs.Error(applicationName, err.Error())
		}
		fmt.Printf("Error: %v\n", err)
		systray.Quit()
	}

	devs, err := devices.GetMIDIDevices(nil)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
	}

	systray.SetTemplateIcon(icon.Data, icon.Data)
	systray.SetTitle(applicationName)
	systray.SetTooltip(applicationName)

	menuexit := make(chan struct{})

	mMIDIMenu := systray.AddMenuItem("MIDI Devices", "List of availalbe MIDI devices")
	midiname := viper.GetString("MidiDevice")
	mMIDIDevices := make([]*systray.MenuItem, 0, len(devs))
	for _, v := range devs {
		mMIDIDevice := mMIDIMenu.AddSubMenuItemCheckbox(v, "", strings.Contains(v, midiname))
		mMIDIDevices = append(mMIDIDevices, mMIDIDevice)
		title := v
		go func() {
			for {
				select {
				case <-mMIDIDevice.ClickedCh:
					for _, v := range mMIDIDevices {
						v.Uncheck()
					}
					mMIDIDevice.Check()
					viper.Set("MidiDevice", title)
					fmt.Println(viper.GetString("MidiDevice"))
					viper.WriteConfig()
					startListeners(title, se)
				case <-menuexit:
					return
				}
			}
		}()
	}

	systray.AddSeparator()

	mQuitItem := systray.AddMenuItem("Quit", "Quit the whole app")
	go func() {
		<-mQuitItem.ClickedCh
		close(quitch)
		close(menuexit)
		systray.Quit()
	}()

	// Instantiate MIDI Controller
	startListeners(midiname, se)
}

// onExit is called by systray on exit and closes the MidiController
func onExit() {
	if mcontrol != nil {
		mcontrol.Close()
	}
}

func main() {
	initSettings()

	systray.Run(onReady, onExit)
}
