package chanSerial

import (
	"errors"
	"fmt"
	"io"
	"time"

	"go.bug.st/serial"
	"salsanci.com/goterm/serafcFilter"
)

// SerialPort - Provides data transfer through a serial port
type SerialPort struct {
	port                           io.ReadWriteCloser
	portName                       string
	baud                           int
	serialResetTimeout             uint
	debug                          bool
	rxExit, txExit, rxDone, txDone chan bool
	inCh                           chan<- []byte
	outCh                          <-chan []byte
	log                            chan<- string
	options                        serial.Mode
}

func NewSerial(portName string, baudrate int, inputCh, outputCh chan []byte, serialResetTimeoutMs uint, afc bool) (*SerialPort, error) {
	var e error
	s := new(SerialPort)
	if afc {
		toPhy := make(chan []byte, 2)
		fromPhy := make(chan []byte, 1)
		serafcFilter.AfcFilter(fromPhy, toPhy, inputCh, outputCh)
		s.inCh = fromPhy
		s.outCh = toPhy
	} else {
		// physical interface
		s.inCh = inputCh   // from serial port
		s.outCh = outputCh // to serial port
	}

	s.rxExit = make(chan bool)
	s.txExit = make(chan bool)
	s.rxDone = make(chan bool)
	s.txDone = make(chan bool)
	s.serialResetTimeout = serialResetTimeoutMs
	s.portName = portName
	s.baud = baudrate
	s.options = serial.Mode{
		BaudRate:          baudrate,
		DataBits:          8,
		StopBits:          serial.OneStopBit,
		Parity:            serial.NoParity,
		InitialStatusBits: &serial.ModemOutputBits{RTS: false, DTR: false},
	}

	if portName != "" {
		s.port, e = serial.Open(s.portName, &s.options)
	} else {
		e = errors.New("blank serial port name")
	}
	if e == nil {
		go s.run()
	}
	return s, e
}
func (s *SerialPort) run() {
	done := make(chan bool, 1)
	for {
		go s.doReceive()
		go s.doSend()
		go func() {
			for {
				select {
				case <-s.txDone:
					s.rxExit <- true
				case <-s.rxDone:
					s.txExit <- true
				}
				done <- true
			}
		}()
		<-done
		time.Sleep(time.Second)
		s.Close()
		wait := 1
		var e error
		if s.log != nil {
			s.log <- "Serial port run: closed"
		}
		for f := true; f; {
			time.Sleep(time.Second * time.Duration(wait))
			if wait < 15 {
				wait++
			}
			if s.log != nil {
				s.log <- "Serial port run: reopening"
			}
			s.port, e = serial.Open(s.portName, &s.options)
			if e != nil {
				if s.log != nil {
					s.log <- fmt.Sprintf("Serial port run: reopening failed [%v]", e)
				}
			} else {
				f = false
				if s.log != nil {
					s.log <- "Serial port run: reopened"
				}
			}
		}
	}
}
func (s *SerialPort) SetLog(l chan<- string) {
	if s.port != nil {
		s.log = l
	}
}
func (s *SerialPort) Close() {
	if s.port != nil {
		s.port.Close()
	}
}

func (s *SerialPort) SetDebug(t bool) {
	s.debug = t
}
func (s *SerialPort) GetDebug() bool {
	return s.debug
}

func (s *SerialPort) doReceive() {
	var err error
	var n, cn int

	buf := make([]byte, 40000)
	lastRec := time.Now()

	for f := true; f; {
		n = 0
		cn, err = s.port.Read(buf[n:])
		n += cn
		for i := 0; f && i < 16 && cn < 8 && err == nil; {
			cn, err = s.port.Read(buf[n:])
			n += cn
			if cn <= 1 && err == nil {
				time.Sleep(100 * time.Microsecond)
			}
			if n > 0 {
				i++
			}
		}
		// if s.log != nil && (n > 0 || err != nil) {
		// 	s.log <- fmt.Sprintf("doReceive: %d [%v]", n, err)
		// }

		select {
		case <-s.txExit:
			f = false
		default:
		}
		if s.serialResetTimeout > 0 && time.Since(lastRec) > time.Millisecond*time.Duration(s.serialResetTimeout) {
			f = false
		}
		if f {
			if n > 0 {
				recBuf := make([]byte, n)
				copy(recBuf, buf[0:n])
				s.inCh <- recBuf
				lastRec = time.Now()
				if s.debug && s.log != nil {
					s.log <- fmt.Sprintf("doReceive: SERIAL RECEIVED [%v][%v] %d bytes [%v]", err, lastRec, n, formatHex(recBuf))
				}
			}

			if err != nil && err != io.EOF {
				if s.log != nil {
					s.log <- fmt.Sprintf("doReceive: SERIAL RECEIVE ERROR [%v], exiting", err)
				}
				f = false
			}
			//if err == io.EOF {
			//	if s.log != nil {
			//		s.log <- fmt.Sprintf("doReceive: SERIAL EOF\n")
			//	}
			//	f = false
			//} else if err != nil {
			//	if s.log != nil {
			//		s.log <- fmt.Sprintf("\r\ndoReceive: SERIAL RECEIVE ERROR [%v], exiting\r\n", err)
			//	}
			//	f = false
			//}
		}
		// time.Sleep(time.Millisecond)
	}
	if s.log != nil {
		s.log <- "doReceive: SERIAL exiting"
	}
	s.rxDone <- true
}

func (s *SerialPort) doSend() {
	for f := true; f; {
		select {
		case val := <-s.outCh:
			n := len(val)
			buf := make([]byte, n)
			copy(buf, val)
			for n := 0; n < len(buf); {
				sz := len(buf) - n
				if sz > 10000 {
					sz = 10000
				}
				sendBuf := buf[n : n+sz]
				n += sz
				if s.debug && s.log != nil {
					s.log <- fmt.Sprintf("doSend: SERIAL SENT %d bytes [%v]", n, formatHex(sendBuf))
				}
				pauseTime := time.Duration(1.05 * float64(time.Second) * float64(len(sendBuf)*10) / float64(s.baud))
				nt := time.NewTimer(pauseTime).C
				n, err := s.port.Write(sendBuf)
				// if s.log != nil {
				// 	ms := float64(pauseTime) / 1000000.0
				// 	s.log <- fmt.Sprintf("doSend: %d [%f] [%v]", n, ms, err)
				// }
				<-nt
				if err != nil {
					if s.log != nil {
						s.log <- fmt.Sprintf("doSend: SERIAL SEND ERROR [%v]", err)
					}
					f = false
				} else if n != len(sendBuf) {
					if s.log != nil {
						s.log <- fmt.Sprintf("doSend: SERIAL SEND ERROR sent[%v] of [%v]", n, len(sendBuf))
					}
					f = false
				}
			}
		case <-s.txExit:
			if s.debug && s.log != nil {
				s.log <- "doSend: SERIAL SEND - Exit request received, exiting send"
			}
			f = false
		}
	}
	s.txDone <- true
}
func formatHex(b []byte) string {
	s := "["
	t := "["
	for i := 0; i < len(b); i++ {
		s += fmt.Sprintf("0x%02X ", b[i])
		if b[i] >= 0x20 && b[i] <= 0x7E {
			t += fmt.Sprintf("%c", b[i])
		} else {
			t += "."
		}
		if (i % 4) == 3 {
			s += "  "
			t += " "
		}
	}
	s += "]"
	t += "]"
	return s + t
}
