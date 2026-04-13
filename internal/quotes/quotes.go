package quotes

import (
	"math/rand"
	"time"
)

// QuitQuotes are shown on exit. Corporate deadpan, retro-future, slightly
// surreal but playful. Loves disclaimers.
var QuitQuotes = []string{
	"Bearing recorded. Navigation session archived.",
	"All headings nominal. Instrument powering down.",
	"Course data saved. The stars will wait.",
	"Pelorus offline. Dead reckoning authorized.",
	"Calibration complete. Operator dismissed.",
	"Thank you for navigating within acceptable parameters.",
	"Session terminated. No files were harmed. Probably.",
	"Your filesystem has been left exactly as confusing as you found it.",
	"Coordinates logged. Magnetic deviation: negligible.",
	"Navigation ceased. Please stow your compass rose.",
	"All bearings relative. All exits temporary.",
	"Instrument secured. Resume manual orientation.",
	"End of watch. File positions are now your responsibility.",
	"The directory tree remembers what you did here.",
	"Pelorus disengaged. You are now navigating by feel.",
	"Session closed per standard operating procedure 7.4.1.",
	"Your heading was adequate. Not exceptional, but adequate.",
	"Files remain where you left them. This is not a guarantee.",
	"Bearing instrument offline. Good luck out there.",
	"Navigation log sealed. Contents subject to audit.",
}

var rng = rand.New(rand.NewSource(time.Now().UnixNano())) //nolint:gosec

// Random returns a random quit quote.
func Random() string {
	return QuitQuotes[rng.Intn(len(QuitQuotes))]
}
