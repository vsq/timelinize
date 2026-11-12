package whatsapp

import (
	"bufio"
	"bytes"
)

var lro = []byte{0xE2, 0x80, 0x8E} // U+200E (LRM)
const (
	dateLen    = 10
	splitTwo   = 2
	scanOffset = 4
)

type messageHeader struct {
	HeaderLen int
	HasLRO    bool
	DateTime  string
	Name      string
}

func chatSplit(data []byte, atEOF bool) (advance int, token []byte, err error) {
	// Start a few bytes in to avoid matching a message start at the very beginning of the buffer
	loc := findMessageStart(data, scanOffset)

	if loc == -1 {
		// We have the last message — return it
		if atEOF {
			return 0, data, bufio.ErrFinalToken
		}
		// We need more data to find a whole message
		return 0, nil, nil
	}

	message := data[:loc]
	return len(message), message, nil
}

// findMessageStart returns the index of the next message start at or after offset, or -1 if none.
func findMessageStart(data []byte, offset int) int {
	for i := offset; i < len(data); i++ {
		c := data[i]
		if c != '[' && (c < '0' || c > '9') && c != lro[0] {
			continue
		}
		if c == lro[0] && !bytes.HasPrefix(data[i:], lro) {
			continue
		}
		if isMessageStart(data[i:]) {
			return i
		}
	}
	return -1
}

func isMessageStart(b []byte) bool {
	_, ok := parseMessageHeader(b)
	return ok
}

func parseMessageHeader(b []byte) (messageHeader, bool) {
	origLen := len(b)

	var h messageHeader
	if bytes.HasPrefix(b, lro) {
		h.HasLRO = true
		b = b[len(lro):]
	}
	if len(b) < dateLen {
		return messageHeader{}, false
	}

	// Standard WhatsApp export:
	// [YYYY/MM/DD, HH:MM(:SS)] Name: message
	if b[0] == '[' {
		endBracket := bytes.IndexByte(b, ']')
		if endBracket == -1 {
			return messageHeader{}, false
		}
		h.DateTime = string(b[1:endBracket])

		nameStart := endBracket + len("] ")
		if nameStart >= len(b) || b[endBracket+1] != ' ' {
			return messageHeader{}, false
		}
		nameEndRel := bytes.Index(b[nameStart:], []byte(": "))
		if nameEndRel <= 0 {
			return messageHeader{}, false
		}
		nameEnd := nameStart + nameEndRel

		h.Name = string(b[nameStart:nameEnd])

		hLen := nameEnd + len(": ")
		if h.HasLRO {
			hLen += len(lro)
		}
		// sanity
		if hLen > origLen {
			return messageHeader{}, false
		}
		h.HeaderLen = hLen
		return h, true
	}

	// Mobile export variant:
	// DD/MM/YYYY, HH:MM(:SS) - Name: message
	parts := bytes.SplitN(b, []byte(" - "), splitTwo)
	if len(parts) != splitTwo {
		return messageHeader{}, false
	}

	h.DateTime = string(parts[0])
	nameAndMsg := parts[1]

	nameEnd := bytes.Index(nameAndMsg, []byte(": "))
	if nameEnd <= 0 {
		return messageHeader{}, false
	}

	h.Name = string(nameAndMsg[:nameEnd])

	hLen := len(h.DateTime) + len(" - ") + nameEnd + len(": ")
	if h.HasLRO {
		hLen += len(lro)
	}
	if hLen > origLen {
		return messageHeader{}, false
	}
	h.HeaderLen = hLen
	return h, true
}
