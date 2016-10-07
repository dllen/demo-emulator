package cpu

import (
	"github.com/drhelius/demo-emulator/gb/memory"
	"github.com/drhelius/demo-emulator/gb/timer"
)

// Interrupt types
const (
	InterruptNone    uint8 = 0x00
	InterruptVBlank  uint8 = 0x01
	InterruptLCDSTAT uint8 = 0x02
	InterruptTimer   uint8 = 0x04
	InterruptSerial  uint8 = 0x08
	InterruptJoypad  uint8 = 0x10
)

const (
	flagZero  uint8 = 0x80
	flagSub   uint8 = 0x40
	flagHalf  uint8 = 0x20
	flagCarry uint8 = 0x10
	flagNone  uint8 = 0x00
)

var (
	af          SixteenBitReg
	bc          SixteenBitReg
	de          SixteenBitReg
	hl          SixteenBitReg
	sp          SixteenBitReg
	pc          SixteenBitReg
	ime         bool
	halt        bool
	branchTaken bool
	clockCycles uint32
)

func init() {
	pc.SetValue(0x0100)
	sp.SetValue(0xFFFE)
	af.SetValue(0x01B0)
	bc.SetValue(0x0013)
	de.SetValue(0x00D8)
	hl.SetValue(0x014D)
}

// Tick runs a single instruction of the processor
// Then returns the number of cycles used
func Tick() uint32 {
	clockCycles = 0

	if halt {
		if interruptPending() != InterruptNone {
			halt = false
		} else {
			clockCycles += 4
		}
	}

	if !halt {
		//fmt.Printf("-> PC: 0x%X  OP: 0x%X\n", pc.GetValue(), memory.Read(pc.GetValue()))
		serveInterrupt(interruptPending())
		runOpcode(fetchOpcode())
	}

	updateTimers()

	return clockCycles
}

// RequestInterrupt is used to raise a new interrupt
func RequestInterrupt(interrupt uint8) {
	memory.Write(0xFF0F, memory.Read(0xFF0F)|interrupt)
}

func fetchOpcode() uint8 {
	opcode := memory.Read(pc.GetValue())
	pc.Increment()
	return opcode
}

func runOpcode(opcode uint8) {
	if opcode == 0xCB {
		opcode = fetchOpcode()
		opcodeCBArray[opcode]()
		clockCycles += machineCyclesCB[opcode]
	} else {
		opcodeArray[opcode]()
		if branchTaken {
			clockCycles += machineCyclesBranched[opcode]
		} else {
			clockCycles += machineCycles[opcode]
		}
	}
}

func interruptIsAboutToRaise() bool {
	ieReg := memory.Read(0xFFFF)
	ifReg := memory.Read(0xFF0F)
	return (ifReg & ieReg & 0x1F) != 0
}

func interruptPending() uint8 {
	ieReg := memory.Read(0xFFFF)
	ifReg := memory.Read(0xFF0F)
	ieIf := ieReg & ifReg

	if (ieIf & 0x01) != 0 {
		return InterruptVBlank
	} else if (ieIf & 0x02) != 0 {
		return InterruptLCDSTAT
	} else if (ieIf & 0x04) != 0 {
		return InterruptTimer
	} else if (ieIf & 0x08) != 0 {
		return InterruptSerial
	} else if (ieIf & 0x10) != 0 {
		return InterruptJoypad
	}

	return InterruptNone
}

func serveInterrupt(interrupt uint8) {
	if ime {
		ifReg := memory.Read(0xFF0F)
		switch interrupt {
		case InterruptVBlank:
			memory.Write(0xFF0F, ifReg&0xFE)
			ime = false
			stackPush(&pc)
			pc.SetValue(0x0040)
			clockCycles += 20
		case InterruptLCDSTAT:
			memory.Write(0xFF0F, ifReg&0xFD)
			ime = false
			stackPush(&pc)
			pc.SetValue(0x0048)
			clockCycles += 20
		case InterruptTimer:
			memory.Write(0xFF0F, ifReg&0xFB)
			ime = false
			stackPush(&pc)
			pc.SetValue(0x0050)
			clockCycles += 20
		case InterruptSerial:
			memory.Write(0xFF0F, ifReg&0xF7)
			ime = false
			stackPush(&pc)
			pc.SetValue(0x0058)
			clockCycles += 20
		case InterruptJoypad:
			memory.Write(0xFF0F, ifReg&0xEF)
			ime = false
			stackPush(&pc)
			pc.SetValue(0x0060)
			clockCycles += 20
		}
	}
}

func updateTimers() {
	timer.DivCycles += clockCycles

	var divCycleTreshold uint32 = 256

	for timer.DivCycles >= divCycleTreshold {
		timer.DivCycles -= divCycleTreshold
		div := memory.Read(0xFF04)
		div++
		memory.Write(0xFF04, div)
	}

	tac := memory.Read(0xFF07)

	// if tima is running
	if (tac & 0x04) != 0 {
		timer.TimaCycles += clockCycles

		var freq uint32

		switch tac & 0x03 {
		case 0:
			freq = 1024
		case 1:
			freq = 16
		case 2:
			freq = 64
		case 3:
			freq = 256
		}

		for timer.TimaCycles >= freq {
			timer.TimaCycles -= freq
			tima := memory.Read(0xFF05)

			if tima == 0xFF {
				tima = memory.Read(0xFF06)
				RequestInterrupt(InterruptTimer)
			} else {
				tima++
			}

			memory.Write(0xFF05, tima)
		}
	}
}
