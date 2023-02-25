# ShuttleMIDI
ShuttleMidi sends MIDI events for the Contour ShuttlExpress to enable usage together with the MIDI Controller feature of [SDR Console](https://www.sdr-radio.com/Console).

ShuttleMIDI doesn't provide it's own MIDI driver for Windows. It relies on the excellent and free [loopMIDI driver from Tobias Erichsen](https://www.tobias-erichsen.de/software/loopmidi.html).

# Installation
1. Install the loopMIDI driver from https://www.tobias-erichsen.de/software/loopmidi.html
2. Add a loopback MIDI port called "ShuttleMIDI" in the loopMIDI control panel
3. Build the application or download the latest release from the ["Releases" section](https://github.com/dg1psi/shuttlemidi/releases).
```
go build -ldflags "-linkmode external -extldflags -static -H=windowsgui" -a
```
4. Run the "shuttlemidi.exe" file
5. Open SDR Console
6. Configure the MIDI Controller in the Options